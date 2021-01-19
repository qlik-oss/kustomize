# Qlik kustomize fork

From: https://github.com/kubernetes-sigs/kustomize

Contains Qlik plugins as part of the executable

## Building
```bash
git clone git@github.com:qlik-oss/kustomize.git kustomize_fork
cd kustomize_fork
make qlik-build-all
```

## Usage
```bash
./kustomize build .
```

## How To Release

To release please use this method only. Dont publish release from UI, that breaks some tests.

```console
git tag qlik/v*.*.*
git push origin qlik/v*.*.*
```
