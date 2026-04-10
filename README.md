# kubectl-crdlist

Kubectl plugin to browse CRDs on Kubernetes

## Install

### Krew

```shell
❯ krew index add os76 https://github.com/xenOs76/krews.git
WARNING: To be able to run kubectl plugins, you need to add
the following to your ~/.bash_profile or ~/.bashrc:

    export PATH="${KREW_ROOT:-$HOME/.krew}/bin:$PATH"

and restart your shell.

WARNING: You have added a new index from "https://github.com/xenOs76/krews.git"
The plugins in this index are not audited for security by the Krew maintainers.
Install them at your own risk.

❯ krew index list
[...]
INDEX     URL
default   https://github.com/kubernetes-sigs/krew-index.git
os76      https://github.com/xenOs76/krews.git

❯ krew update
[...]
Updated the local copy of plugin index.
Updated the local copy of plugin index "os76".

❯ krew search crdlist
[...]
NAME              DESCRIPTION                                         INSTALLED
os76/crdlist      kubectl-crdlist, CRD visualization plugin for K...  no

❯ krew install os76/crdlist
[...]
Updated the local copy of plugin index.
Updated the local copy of plugin index "os76".
Installing plugin: crdlist
Installed plugin: crdlist
\
 | Use this plugin:
 |      kubectl crdlist
 | Documentation:
 |      https://github.com/xenOs76/kubectl-crdlist
/
```

### Homebrew

```shell
> brew tap xenos76/tap

> brew install --casks kubectl-crdlist
```
