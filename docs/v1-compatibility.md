# v1 Compatibility

The shell shim `completions/alogin.zsh` provides backward-compatible functions:

```
t            → alogin access
r            → alogin access --auto-gw
s            → alogin sftp
f            → alogin ftp
m            → alogin mount
ct           → alogin cluster
cr           → alogin cluster --gateway
addsvr       → alogin compute add
delsvr       → alogin compute delete
dissvr       → alogin compute show
dissvrlist   → alogin compute list
chgsvr       → alogin compute update
chgpwd       → alogin compute passwd
addalias     → alogin alias add
disalias     → alogin alias show
tver         → alogin version
```
