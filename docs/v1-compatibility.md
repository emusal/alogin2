# v1 Compatibility

The shell shim `completions/alogin.zsh` provides backward-compatible functions:

```
t            → alogin connect
r            → alogin connect --auto-gw
s            → alogin sftp
f            → alogin ftp
m            → alogin mount
ct           → alogin cluster
cr           → alogin cluster --gateway
addsvr       → alogin server add
delsvr       → alogin server delete
dissvr       → alogin server show
dissvrlist   → alogin server list
chgsvr       → alogin server update
chgpwd       → alogin server passwd
addalias     → alogin alias add
disalias     → alogin alias show
tver         → alogin version
```
