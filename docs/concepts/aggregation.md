# Aggregation

While the API servers created by the apiserver-builder can be accessed
directly, it is most convinient to use them with the Kubernetes API server
aggregator.  This allows multiple APIs in the cluster to appear as if they
were being served by a single API server, so that cluster components, and
kubectl, can continue communicating as normal without special logic to
discover the locations of the different APIs.

In this document, we'll refer to API servers generated with
apiserver-builder as *addon API servers*.  We'll speak in terms of
a sample addon API server which serves an API group called `wardle`, with
an single namespaced resource `flunders`, abbreviated `fl`.

Understanding how the aggregator works with addon API servers requires
understanding how [authentication and authorization](./auth.md) works.  If
you have not yet read that section, please do.

## Enabling the Aggregator

The API aggregator is integrated into the main Kubernetes API server in
Kubernetes 1.7+, but must be run as a separate pod in Kubernetes 1.6.  In
Kubernetes 1.7+, it is only served on the secure port of the API server.

In Kubernetes 1.7, in order for the aggregator to work properly, it needs
to have the appropriate certificates.  First, ensure that you have
RequestHeader CA certificates, and client certificates signed by that CA,
as discussed in the [authentication and authorization](./auth.md) section.

These certificates must be passed to the main Kubernetes API server using
the flags `--proxy-client-cert-file` and `--proxy-client-key-file`.  This
allows the aggregator to identify itself when making requests, so that
addon API servers can use its delegated RequestHeader authentication.

Enabling the aggregator in 1.6 is outside the scope of this document.  The
aggregator directory itself provides examples on how to do so in the
Kubernetes 1.6 release.

## kubectl and Discovery

When `kubectl get fl` is called, `kubectl` does not initially know what an
`fl` is, or how to get one.  In order to determine this information, it
uses a mechanism called *discovery*.

All Kubernetes API servers serve discovery information.  To get this
information, `kubectl` first queries `https://$SERVER/apis`, which lists
all available API groups and versions, as well as the *preferred version*.
Then, for each of these API groups and versions, it queries
`https://$SERVER/apis/$GROUP/$VERSION`.  This returns a list of resources,
along with whether the resource is *namespaced*, which operations (e.g.
`get` or `list`) it supports, as well as any short names (like `fl` for
`fluders`).

Once `kubectl` has retrieved the entire set of available resources for
a particular cluster, it can then determine how to operate on the exposed
resources.  For example, in our case above, `kubectl` determines that `fl`
is a shortname for `flunders`, and that `fluders` are a namespaced
resource that's part of the API group `wardle`, which has a preferred
version of `v1alpha`.  Thus, `kubectl` will attempt a query of
`https://$SERVER/apis/wardle/v1alpha1/namespaces/$NS/flunders/`.

Since querying all of these discovery endpoints for every kubectl request
would be expensive, `kubectl` caches this information in the same folder
as where your kubeconfig goes, based on the name of the cluster to which
you are connecting. Clearing this cache can resolve issues with not being
able to use `kubectl` to fetch resources from newly registered APIs.

Controller managers also commonly use discovery information to determine
whether or not they should run: if the resources that they require are not
present of the cluster, controller managers can just fail to start.

You can see the list of API group versions returned by discover using the
`kubectl api-versions` command.

## Registering APIs

In order for your API to appear in the discovery information served by the
aggregator, it must be registered with the aggregator.  In order to do
this, the aggregator exposes a normal Kubernetes API group called
`apiregistration.k8s.io`, with a single resource, APIService.

Each APIService corresponds to a single group-version, and different
versions of a single API can be backed by different APIService objects.

Let's take a look at the APIService for `wardle/v1alpha1`, using `kubectl
get apiservice v1alpha1.wardle -o yaml`:

```yaml
apiVersion: apiregistration.k8s.io/v1beta1
kind: APIService
metadata:
  name: v1alpha1.wardle
  ...
spec:
  caBundle: <base64-encoded-serving-ca-certificate>
  group: wardle
  version: v1alpha1
  priority: 100
  service:
    name: wardle-server
    namespace: wardle-namespace
status:
  ...
```

Notice that this is a Kubernetes API object like any other: it has a name,
a spec, a status, etc.  There are several important fields in the spec.

The first important field is `caBundle`.  This is the base64-encoding
version of a CA certificate that can be used to verify the serving
certificates of the API server.  The aggregator will check that the
serving certificates are for a hostname of `<service>.<namespace>.svc` (in
the case of the APIService above, that's
`wardle-server.wardle-namespace.svc`).

Next are the `group` and `version` fields.  These determine which
group-version the APIService describes.

The `priority` field indicates how the aggregator sorts and prioritizes
discovery information.

Finally, the `service` field determines how the aggregator actually
connects to the addon API server.  It will look up the service IP of the
service described, and connect to that.  As mentioned above, however, it
validates certificates based on hostname, so your certificates do not
necessarily need to include the IP address of the service.

## Proxying

In addition to serving discovery information and registering API groups
and servers, the aggregator acts as proxy.

The aggregator "natively" serves discovery information that lets us list
the available API groups, and the corresponding versions.  For other
requests, such as fetching the available resources in a group-version, or
making a request against an API, the aggregator contacts a registered API
server.

When the aggregator receives a request that it needs to proxy, it first
performs authentication (but not authorization) using the authentication
methods configured for the main Kubernetes API server.  Once it has
completed authentication, it records the authentication information in
headers, and forwards the request to the appropriate addon API server.

For instance, suppose we make a request

```
GET /apis/wardle/v1alpha1/namespaces/somens/fluders/foo
```

using the admin client certificates.  The aggregator will verify the
certificates, strip them from the request, and add the `X-Remote-User:
system:admin` header.

The aggregator will the connect to the wardle server, verifying the wardle
server's service certificates using the CA certificate from the `caBundle`
field of the APIService object for `wardle/v1alpha1`, and submitting it's
own proxy client certificates to identify itself to the wardle server.

The wardle server will receive the modified request, verify the proxy
client certificates against it's requestheader CA certificate, and treat
the request as if it had come from the `system:admin` user, as marked in
the `X-Remote-User` header.  The wardle server can then proceed along with
its normal serving logic, validating authorization and returning a result
to the aggregator.  The aggregator then returns the result back to us.
