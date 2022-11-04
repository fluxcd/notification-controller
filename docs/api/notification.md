<h1>Notification API reference</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#notification.toolkit.fluxcd.io%2fv1beta2">notification.toolkit.fluxcd.io/v1beta2</a>
</li>
</ul>
<h2 id="notification.toolkit.fluxcd.io/v1beta2">notification.toolkit.fluxcd.io/v1beta2</h2>
<p>Package v1beta2 contains API Schema definitions for the notification v1beta2 API group.</p>
Resource Types:
<ul class="simple"><li>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Alert">Alert</a>
</li><li>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Provider">Provider</a>
</li><li>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Receiver">Receiver</a>
</li></ul>
<h3 id="notification.toolkit.fluxcd.io/v1beta2.Alert">Alert
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
<code>notification.toolkit.fluxcd.io/v1beta2</code>
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
<a href="#notification.toolkit.fluxcd.io/v1beta2.AlertSpec">
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
<p>ProviderRef specifies which Provider this Alert should use.</p>
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
<p>EventSeverity specifies how to filter events based on severity.
If set to &lsquo;info&rsquo; no events will be filtered.</p>
</td>
</tr>
<tr>
<td>
<code>eventSources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>EventSources specifies how to filter events based
on the involved object kind, name and namespace.</p>
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
<p>ExclusionList specifies a list of Golang regular expressions
to be used for excluding messages.</p>
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
<p>Summary holds a short description of the impact and affected cluster.</p>
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
<p>Suspend tells the controller to suspend subsequent
events handling for this Alert.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.AlertStatus">
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
<h3 id="notification.toolkit.fluxcd.io/v1beta2.Provider">Provider
</h3>
<p>Provider is the Schema for the providers API.</p>
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
<code>notification.toolkit.fluxcd.io/v1beta2</code>
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
<a href="#notification.toolkit.fluxcd.io/v1beta2.ProviderSpec">
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
<p>Type specifies which Provider implementation to use.</p>
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
<p>Channel specifies the destination channel where events should be posted.</p>
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
<p>Username specifies the name under which events are posted.</p>
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
<p>Address specifies the HTTP/S incoming webhook address of this Provider.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout for sending alerts to the Provider.</p>
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
<p>Proxy the HTTP/S address of the proxy server.</p>
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
<p>SecretRef specifies the Secret containing the authentication
credentials for this Provider.</p>
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
<p>CertSecretRef specifies the Secret containing
a PEM-encoded CA certificate (<code>caFile</code>).</p>
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
<p>Suspend tells the controller to suspend subsequent
events handling for this Provider.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.ProviderStatus">
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
<h3 id="notification.toolkit.fluxcd.io/v1beta2.Receiver">Receiver
</h3>
<p>Receiver is the Schema for the receivers API.</p>
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
<code>notification.toolkit.fluxcd.io/v1beta2</code>
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
<a href="#notification.toolkit.fluxcd.io/v1beta2.ReceiverSpec">
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
<p>Events specifies the list of event types to handle,
e.g. &lsquo;push&rsquo; for GitHub or &lsquo;Push Hook&rsquo; for GitLab.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.CrossNamespaceObjectReference">
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
<p>SecretRef specifies the Secret containing the token used
to validate the payload authenticity.</p>
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
<p>Suspend tells the controller to suspend subsequent
events handling for this receiver.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.ReceiverStatus">
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
<h3 id="notification.toolkit.fluxcd.io/v1beta2.AlertSpec">AlertSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Alert">Alert</a>)
</p>
<p>AlertSpec defines an alerting rule for events involving a list of objects.</p>
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
<p>ProviderRef specifies which Provider this Alert should use.</p>
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
<p>EventSeverity specifies how to filter events based on severity.
If set to &lsquo;info&rsquo; no events will be filtered.</p>
</td>
</tr>
<tr>
<td>
<code>eventSources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>EventSources specifies how to filter events based
on the involved object kind, name and namespace.</p>
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
<p>ExclusionList specifies a list of Golang regular expressions
to be used for excluding messages.</p>
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
<p>Summary holds a short description of the impact and affected cluster.</p>
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
<p>Suspend tells the controller to suspend subsequent
events handling for this Alert.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta2.AlertStatus">AlertStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Alert">Alert</a>)
</p>
<p>AlertStatus defines the observed state of the Alert.</p>
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
<code>ReconcileRequestStatus</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#ReconcileRequestStatus">
github.com/fluxcd/pkg/apis/meta.ReconcileRequestStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>ReconcileRequestStatus</code> are embedded into this type.)
</p>
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
<p>Conditions holds the conditions for the Alert.</p>
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
<h3 id="notification.toolkit.fluxcd.io/v1beta2.CrossNamespaceObjectReference">CrossNamespaceObjectReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.AlertSpec">AlertSpec</a>, 
<a href="#notification.toolkit.fluxcd.io/v1beta2.ReceiverSpec">ReceiverSpec</a>)
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
<p>API version of the referent.</p>
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
<p>Kind of the referent.</p>
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
<p>Name of the referent.</p>
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
<p>Namespace of the referent.</p>
</td>
</tr>
<tr>
<td>
<code>matchLabels</code><br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>MatchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
map is equivalent to an element of matchExpressions, whose key field is &ldquo;key&rdquo;, the
operator is &ldquo;In&rdquo;, and the values array contains only &ldquo;value&rdquo;. The requirements are ANDed.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta2.ProviderSpec">ProviderSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Provider">Provider</a>)
</p>
<p>ProviderSpec defines the desired state of the Provider.</p>
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
<p>Type specifies which Provider implementation to use.</p>
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
<p>Channel specifies the destination channel where events should be posted.</p>
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
<p>Username specifies the name under which events are posted.</p>
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
<p>Address specifies the HTTP/S incoming webhook address of this Provider.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout for sending alerts to the Provider.</p>
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
<p>Proxy the HTTP/S address of the proxy server.</p>
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
<p>SecretRef specifies the Secret containing the authentication
credentials for this Provider.</p>
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
<p>CertSecretRef specifies the Secret containing
a PEM-encoded CA certificate (<code>caFile</code>).</p>
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
<p>Suspend tells the controller to suspend subsequent
events handling for this Provider.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta2.ProviderStatus">ProviderStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Provider">Provider</a>)
</p>
<p>ProviderStatus defines the observed state of the Provider.</p>
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
<code>ReconcileRequestStatus</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#ReconcileRequestStatus">
github.com/fluxcd/pkg/apis/meta.ReconcileRequestStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>ReconcileRequestStatus</code> are embedded into this type.)
</p>
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
<p>Conditions holds the conditions for the Provider.</p>
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
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta2.ReceiverSpec">ReceiverSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Receiver">Receiver</a>)
</p>
<p>ReceiverSpec defines the desired state of the Receiver.</p>
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
<p>Events specifies the list of event types to handle,
e.g. &lsquo;push&rsquo; for GitHub or &lsquo;Push Hook&rsquo; for GitLab.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br>
<em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.CrossNamespaceObjectReference">
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
<p>SecretRef specifies the Secret containing the token used
to validate the payload authenticity.</p>
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
<p>Suspend tells the controller to suspend subsequent
events handling for this receiver.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1beta2.ReceiverStatus">ReceiverStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1beta2.Receiver">Receiver</a>)
</p>
<p>ReceiverStatus defines the observed state of the Receiver.</p>
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
<code>ReconcileRequestStatus</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#ReconcileRequestStatus">
github.com/fluxcd/pkg/apis/meta.ReconcileRequestStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>ReconcileRequestStatus</code> are embedded into this type.)
</p>
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
<p>Conditions holds the conditions for the Receiver.</p>
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
<p>URL is the generated incoming webhook address in the format
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
<p>ObservedGeneration is the last observed generation of the Receiver object.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
