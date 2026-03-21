package vault

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// AgeVault stores secrets in an age-encrypted JSON file.
// The master password is used as a passphrase-based age recipient.
type AgeVault struct {
	path       string
	passphrase string
}

// NewAge creates an AgeVault at the given path, unlocked with passphrase.
func NewAge(path, passphrase string) *AgeVault {
	return &AgeVault{path: path, passphrase: passphrase}
}

func (v *AgeVault) Name() string { return "age" }

func (v *AgeVault) Get(account string) (string, error) {
	m, err := v.load()
	if err != nil {
		return "", err
	}
	pwd, ok := m[account]
	if !ok {
		return "", ErrNotFound
	}
	return pwd, nil
}

func (v *AgeVault) Set(account, password string) error {
	m, err := v.load()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if m == nil {
		m = make(map[string]string)
	}
	m[account] = password
	return v.save(m)
}

func (v *AgeVault) Delete(account string) error {
	m, err := v.load()
	if err != nil {
		return err
	}
	delete(m, account)
	return v.save(m)
}

func (v *AgeVault) load() (map[string]string, error) {
	f, err := os.Open(v.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	id, err := age.NewScryptIdentity(v.passphrase)
	if err != nil {
		return nil, fmt.Errorf("age identity: %w", err)
	}

	ar := armor.NewReader(f)
	r, err := age.Decrypt(ar, id)
	if err != nil {
		return nil, fmt.Errorf("age decrypt: %w", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (v *AgeVault) save(m map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(v.path), 0700); err != nil {
		return err
	}

	rec, err := age.NewScryptRecipient(v.passphrase)
	if err != nil {
		return fmt.Errorf("age recipient: %w", err)
	}

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	tmp := v.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	aw := armor.NewWriter(f)
	w, err := age.Encrypt(aw, rec)
	if err != nil {
		f.Close()
		return fmt.Errorf("age encrypt: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := w.Close(); err != nil {
		f.Close()
		return err
	}
	if err := aw.Close(); err != nil {
		f.Close()
		return err
	}
	f.Close()

	return os.Rename(tmp, v.path)
}
