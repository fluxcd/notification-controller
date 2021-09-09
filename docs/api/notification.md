<h1>Notification API reference</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#notification.toolkit.fluxcd.io%2fv1beta1">notification.toolkit.fluxcd.io/v1beta1</a>
</li>
</ul>
<h2 id="notification.toolkit.fluxcd.io/v1beta1">notification.toolkit.fluxcd.io/v1beta1</h2>
<p>Package v1beta1 contains API Schema definitions for the notification v1beta1 API group</p>
Resource Types:
<ul class="simple"><li>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Alert">Alert</a>
</li><li>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Provider">Provider</a>
</li><li>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Receiver">Receiver</a>
</li></ul>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.Alert">Alert
</h3>
<p>Alert is the Schema for the alerts API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
string</td>
<td>
<code>notification.toolkit.fluxcd.io/v1beta1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
string
</td>
<td>
<code>Alert</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.AlertSpec">
AlertSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>providerRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Send events using this provider.</p>
</td>
</tr>
<tr>
<td>
<code>eventSeverity</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Filter events based on severity, defaults to (&lsquo;info&rsquo;).
If set to &lsquo;info&rsquo; no events will be filtered.</p>
</td>
</tr>
<tr>
<td>
<code>eventSources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>Filter events based on the involved objects.</p>
</td>
</tr>
<tr>
<td>
<code>exclusionList</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of Golang regular expressions to be used for excluding messages.</p>
</td>
</tr>
<tr>
<td>
<code>summary</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Short description of the impact and affected cluster.</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent events dispatching.
Defaults to false.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.AlertStatus">
AlertStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.Provider">Provider
</h3>
<p>Provider is the Schema for the providers API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
string</td>
<td>
<code>notification.toolkit.fluxcd.io/v1beta1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
string
</td>
<td>
<code>Provider</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.ProviderSpec">
ProviderSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>type</code><br>
<em>
string
</em>
</td>
<td>
<p>Type of provider</p>
</td>
</tr>
<tr>
<td>
<code>channel</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alert channel for this provider</p>
</td>
</tr>
<tr>
<td>
<code>username</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bot username for this provider</p>
</td>
</tr>
<tr>
<td>
<code>address</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTP/S webhook address of this provider</p>
</td>
</tr>
<tr>
<td>
<code>proxy</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTP/S address of the proxy</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secret reference containing the provider webhook URL
using &ldquo;address&rdquo; as data key</p>
</td>
</tr>
<tr>
<td>
<code>certSecretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertSecretRef can be given the name of a secret containing
a PEM-encoded CA certificate (<code>caFile</code>)</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent events handling.
Defaults to false.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.ProviderStatus">
ProviderStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.Receiver">Receiver
</h3>
<p>Receiver is the Schema for the receivers API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
string</td>
<td>
<code>notification.toolkit.fluxcd.io/v1beta1</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
string
</td>
<td>
<code>Receiver</code>
</td>
</tr>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.ReceiverSpec">
ReceiverSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>type</code><br>
<em>
string
</em>
</td>
<td>
<p>Type of webhook sender, used to determine
the validation procedure and payload deserialization.</p>
</td>
</tr>
<tr>
<td>
<code>events</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of events to handle,
e.g. &lsquo;push&rsquo; for GitHub or &lsquo;Push Hook&rsquo; for GitLab.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>A list of resources to be notified about changes.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Secret reference containing the token used
to validate the payload authenticity</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent events handling.
Defaults to false.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.ReceiverStatus">
ReceiverStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.AlertSpec">AlertSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Alert">Alert</a>)
</p>
<p>AlertSpec defines an alerting rule for events involving a list of objects</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>providerRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Send events using this provider.</p>
</td>
</tr>
<tr>
<td>
<code>eventSeverity</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Filter events based on severity, defaults to (&lsquo;info&rsquo;).
If set to &lsquo;info&rsquo; no events will be filtered.</p>
</td>
</tr>
<tr>
<td>
<code>eventSources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>Filter events based on the involved objects.</p>
</td>
</tr>
<tr>
<td>
<code>exclusionList</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of Golang regular expressions to be used for excluding messages.</p>
</td>
</tr>
<tr>
<td>
<code>summary</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Short description of the impact and affected cluster.</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent events dispatching.
Defaults to false.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.AlertStatus">AlertStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Alert">Alert</a>)
</p>
<p>AlertStatus defines the observed state of Alert</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last observed generation.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.CrossNamespaceObjectReference">CrossNamespaceObjectReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.AlertSpec">AlertSpec</a>, 
<a href="#notification.toolkit.fluxcd.io/v1beta1.ReceiverSpec">ReceiverSpec</a>)
</p>
<p>CrossNamespaceObjectReference contains enough information to let you locate the
typed referenced object at cluster level</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent</p>
</td>
</tr>
<tr>
<td>
<code>kind</code><br>
<em>
string
</em>
</td>
<td>
<p>Kind of the referent</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Name of the referent</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Namespace of the referent</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.ProviderSpec">ProviderSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Provider">Provider</a>)
</p>
<p>ProviderSpec defines the desired state of Provider</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br>
<em>
string
</em>
</td>
<td>
<p>Type of provider</p>
</td>
</tr>
<tr>
<td>
<code>channel</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alert channel for this provider</p>
</td>
</tr>
<tr>
<td>
<code>username</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bot username for this provider</p>
</td>
</tr>
<tr>
<td>
<code>address</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTP/S webhook address of this provider</p>
</td>
</tr>
<tr>
<td>
<code>proxy</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTP/S address of the proxy</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secret reference containing the provider webhook URL
using &ldquo;address&rdquo; as data key</p>
</td>
</tr>
<tr>
<td>
<code>certSecretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertSecretRef can be given the name of a secret containing
a PEM-encoded CA certificate (<code>caFile</code>)</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent events handling.
Defaults to false.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.ProviderStatus">ProviderStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Provider">Provider</a>)
</p>
<p>ProviderStatus defines the observed state of Provider</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.ReceiverSpec">ReceiverSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Receiver">Receiver</a>)
</p>
<p>ReceiverSpec defines the desired state of Receiver</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br>
<em>
string
</em>
</td>
<td>
<p>Type of webhook sender, used to determine
the validation procedure and payload deserialization.</p>
</td>
</tr>
<tr>
<td>
<code>events</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of events to handle,
e.g. &lsquo;push&rsquo; for GitHub or &lsquo;Push Hook&rsquo; for GitLab.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>A list of resources to be notified about changes.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Secret reference containing the token used
to validate the payload authenticity</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent events handling.
Defaults to false.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta1.ReceiverStatus">ReceiverStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta1.Receiver">Receiver</a>)
</p>
<p>ReceiverStatus defines the observed state of Receiver</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>url</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Generated webhook URL in the format
of &lsquo;/hook/sha256sum(token+name+namespace)&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last observed generation.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
