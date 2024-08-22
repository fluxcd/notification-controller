<h1>Notification API reference v1</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#notification.toolkit.fluxcd.io%2fv1">notification.toolkit.fluxcd.io/v1</a>
</li>
</ul>
<h2 id="notification.toolkit.fluxcd.io/v1">notification.toolkit.fluxcd.io/v1</h2>
<p>Package v1 contains API Schema definitions for the notification v1 API group.</p>
Resource Types:
<ul class="simple"><li>
<a href="#notification.toolkit.fluxcd.io/v1.Receiver">Receiver</a>
</li></ul>
<h3 id="notification.toolkit.fluxcd.io/v1.Receiver">Receiver
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
<code>notification.toolkit.fluxcd.io/v1</code>
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
<a href="#notification.toolkit.fluxcd.io/v1.ReceiverSpec">
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
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Interval at which to reconcile the Receiver with its Secret references.</p>
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
<a href="#notification.toolkit.fluxcd.io/v1.CrossNamespaceObjectReference">
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
<code>resourceExpressions</code><br>
<em>
[]string
</em>
</td>
<td>
<p>ResourceExpressions is a list of CEL expressions that will be parsed to
determine resources to be notified about changes.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
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
<a href="#notification.toolkit.fluxcd.io/v1.ReceiverStatus">
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
<h3 id="notification.toolkit.fluxcd.io/v1.CrossNamespaceObjectReference">CrossNamespaceObjectReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1.ReceiverSpec">ReceiverSpec</a>)
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
<p>Name of the referent
If multiple resources are targeted <code>*</code> may be set.</p>
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
operator is &ldquo;In&rdquo;, and the values array contains only &ldquo;value&rdquo;. The requirements are ANDed.
MatchLabels requires the name to be set to <code>*</code>.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.toolkit.fluxcd.io/v1.ReceiverSpec">ReceiverSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1.Receiver">Receiver</a>)
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
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Interval at which to reconcile the Receiver with its Secret references.</p>
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
<a href="#notification.toolkit.fluxcd.io/v1.CrossNamespaceObjectReference">
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
<code>resourceExpressions</code><br>
<em>
[]string
</em>
</td>
<td>
<p>ResourceExpressions is a list of CEL expressions that will be parsed to
determine resources to be notified about changes.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
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
<h3 id="notification.toolkit.fluxcd.io/v1.ReceiverStatus">ReceiverStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.toolkit.fluxcd.io/v1.Receiver">Receiver</a>)
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
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#ReconcileRequestStatus">
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
<code>webhookPath</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>WebhookPath is the generated incoming webhook address in the format
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
