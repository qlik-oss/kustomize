# Qlik kustomize fork

From: https://github.com/kubernetes-sigs/kustomize

Contains Qlik plugins as part of the executable

## Building
```bash
git clone git@github.com:qlik-oss/kustomize.git kustomize_fork
cd kustomize_fork/kustomize
go install .
```

<<<<<<< HEAD
The YAML can be directly [applied] to a cluster:

> ```
> kustomize build ~/someApp | kubectl apply -f -
> ```


### 2) Create [variants] using [overlays]

Manage traditional [variants] of a configuration - like
_development_, _staging_ and _production_ - using
[overlays] that modify a common [base].

![overlay image][imageOverlay]

File structure:
> ```
> ~/someApp
> ├── base
> │   ├── deployment.yaml
> │   ├── kustomization.yaml
> │   └── service.yaml
> └── overlays
>     ├── development
>     │   ├── cpu_count.yaml
>     │   ├── kustomization.yaml
>     │   └── replica_count.yaml
>     └── production
>         ├── cpu_count.yaml
>         ├── kustomization.yaml
>         └── replica_count.yaml
> ```

Take the work from step (1) above, move it into a
`someApp` subdirectory called `base`, then
place overlays in a sibling directory.

An overlay is just another kustomization, referring to
the base, and referring to patches to apply to that
base.

This arrangement makes it easy to manage your
configuration with `git`.  The base could have files
from an upstream repository managed by someone else.
The overlays could be in a repository you own.
Arranging the repo clones as siblings on disk avoids
the need for git submodules (though that works fine, if
you are a submodule fan).

Generate YAML with

```sh
kustomize build ~/someApp/overlays/production
```

The YAML can be directly [applied] to a cluster:

> ```sh
> kustomize build ~/someApp/overlays/production | kubectl apply -f -
> ```

## Community

To file bugs please read [this](docs/bugs.md).

Before working on an implementation, please

 * Read the [eschewed feature list].
 * File an issue describing
   how the new feature would behave
   and label it [kind/feature].

### Other communication channels

- [Slack]
- [Mailing List]
- General kubernetes [community page]

### Code of conduct

Participation in the Kubernetes community
is governed by the [Kubernetes Code of Conduct].

[`make`]: https://www.gnu.org/software/make
[`sed`]: https://www.gnu.org/software/sed
[DAM]: docs/glossary.md#declarative-application-management
[KEP]: https://github.com/kubernetes/enhancements/blob/master/keps/sig-cli/0008-kustomize.md
[Kubernetes Code of Conduct]: code-of-conduct.md
[Mailing List]: https://groups.google.com/forum/#!forum/kubernetes-sig-cli
[Slack]: https://kubernetes.slack.com/messages/sig-cli
[applied]: docs/glossary.md#apply
[base]: docs/glossary.md#base
[community page]: http://kubernetes.io/community/
[declarative configuration]: docs/glossary.md#declarative-application-management
[eschewed feature list]: docs/eschewedFeatures.md
[imageBase]: docs/images/base.jpg
[imageOverlay]: docs/images/overlay.jpg
[kind/feature]: /../../labels/kind%2Ffeature
[kubectl announcement]: https://kubernetes.io/blog/2019/03/25/kubernetes-1-14-release-announcement
[kubectl book]: https://kubectl.docs.kubernetes.io/pages/app_customization/introduction.html
[kubernetes documentation]: https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/
[kubernetes style]: docs/glossary.md#kubernetes-style-object
[kustomization]: docs/glossary.md#kustomization
[overlay]: docs/glossary.md#overlay
[overlays]: docs/glossary.md#overlay
[release page]: /../../releases
[resource]: docs/glossary.md#resource
[resources]: docs/glossary.md#resource
[sig-cli]: https://github.com/kubernetes/community/blob/master/sig-cli/README.md
[variant]: docs/glossary.md#variant
[variants]: docs/glossary.md#variant
[v2.0.3]: /../../releases/tag/v2.0.3
[v2.1.0]: /../../releases/tag/v2.1.0
[workflows]: docs/workflows.md
=======
## Usage
```bash
$HOME/go/bin/kustomize build . --load_restrictor=none
```
>>>>>>> Updating Readme
