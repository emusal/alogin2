package tui

import (
	"os"
	"strings"
)

// isKorean returns true when the OS locale is Korean (checks LC_ALL, LC_MESSAGES, LANG).
func isKorean() bool {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			return strings.HasPrefix(strings.ToLower(v), "ko")
		}
	}
	return false
}

type descPair struct{ ko, en [2]string }

// pageDescs holds two-line descriptions for each TUI page.
var pageDescs = map[string]descPair{
	"server": {
		ko: [2]string{
			"등록된 SSH 서버를 검색하고 접속합니다.",
			"Enter 다이렉트 접속  ·  r 게이트웨이 경유  ·  Tab 상세보기  ·  a/e/d 추가·편집·삭제",
		},
		en: [2]string{
			"Browse and connect to registered SSH servers.",
			"Enter to connect  ·  r via gateway  ·  Tab for details  ·  a/e/d add·edit·delete",
		},
	},
	"gateway": {
		ko: [2]string{
			"배스천 호스트를 통한 다중 홉 SSH 경로를 관리합니다.",
			"서버에 게이트웨이를 지정하면 서버 목록에서 r 키로 경유 접속합니다.",
		},
		en: [2]string{
			"Manage multi-hop SSH routes via bastion hosts.",
			"Assign a gateway to a server; press r in the server list to route through it.",
		},
	},
	"cluster": {
		ko: [2]string{
			"여러 서버에 동시 접속하는 클러스터를 관리합니다.",
			"실행은 alogin cluster <name>  ·  tmux / iTerm2 / Terminal.app 지원",
		},
		en: [2]string{
			"Manage server groups for simultaneous multi-host SSH sessions.",
			"Launch with: alogin cluster <name>  ·  tmux / iTerm2 / Terminal.app supported",
		},
	},
	"hosts": {
		ko: [2]string{
			"DNS 대신 사용할 로컬 호스트 이름 → IP 매핑을 관리합니다.",
			"서버 접속 시 이 목록에서 먼저 IP를 조회합니다. /etc/hosts와 유사하지만 alogin 전용입니다.",
		},
		en: [2]string{
			"Manage hostname → IP mappings used instead of DNS.",
			"alogin resolves names from this list first. Similar to /etc/hosts but scoped to alogin.",
		},
	},
}

// pageDesc returns the two description lines for the given page key.
func pageDesc(key string) (line1, line2 string) {
	d, ok := pageDescs[key]
	if !ok {
		return "", ""
	}
	if isKorean() {
		return d.ko[0], d.ko[1]
	}
	return d.en[0], d.en[1]
}
