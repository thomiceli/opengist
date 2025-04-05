# Install on Kubernetes

A [Helm](https://helm.sh) chart is available to install Opengist on a Kubernetes cluster.
Check the [Helm documentation](https://helm.sh/docs/) for more information on how to use Helm.

A non-customized installation of Opengist can be done with:

```bash
helm repo add opengist https://helm.opengist.io
 
helm install opengist opengist/opengist
```

Refer to the [Opengist chart](https://github.com/thomiceli/opengist/tree/master/helm/opengist) for more information 
about the chart and to customize your installation.