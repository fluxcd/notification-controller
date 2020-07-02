<h1>Kustomize API reference</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#notification.fluxcd.io%2fv1alpha1">notification.fluxcd.io/v1alpha1</a>
</li>
</ul>
<h2 id="notification.fluxcd.io/v1alpha1">notification.fluxcd.io/v1alpha1</h2>
<p>Package v1alpha1 contains API Schema definitions for the notification v1alpha1 API group</p>
Resource Types:
<ul class="simple"><li>
<a href="#notification.fluxcd.io/v1alpha1.Alert">Alert</a>
</li><li>
<a href="#notification.fluxcd.io/v1alpha1.Provider">Provider</a>
</li></ul>
<h3 id="notification.fluxcd.io/v1alpha1.Alert">Alert
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
<code>notification.fluxcd.io/v1alpha1</code>
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
<a href="#notification.fluxcd.io/v1alpha1.AlertSpec">
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Send events using this provider</p>
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
<p>Filter events based on severity, defaults to (&lsquo;info&rsquo;).</p>
</td>
</tr>
<tr>
<td>
<code>eventSources</code><br>
<em>
<a href="#notification.fluxcd.io/v1alpha1.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>Filter events based on the involved objects</p>
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
<a href="#notification.fluxcd.io/v1alpha1.AlertStatus">
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
<h3 id="notification.fluxcd.io/v1alpha1.Provider">Provider
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
<code>notification.fluxcd.io/v1alpha1</code>
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
<a href="#notification.fluxcd.io/v1alpha1.ProviderSpec">
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
<p>HTTP(S) webhook address of this provider</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secret reference containing the provider webhook URL</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#notification.fluxcd.io/v1alpha1.ProviderStatus">
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
<h3 id="notification.fluxcd.io/v1alpha1.AlertSpec">AlertSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.fluxcd.io/v1alpha1.Alert">Alert</a>)
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
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Send events using this provider</p>
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
<p>Filter events based on severity, defaults to (&lsquo;info&rsquo;).</p>
</td>
</tr>
<tr>
<td>
<code>eventSources</code><br>
<em>
<a href="#notification.fluxcd.io/v1alpha1.CrossNamespaceObjectReference">
[]CrossNamespaceObjectReference
</a>
</em>
</td>
<td>
<p>Filter events based on the involved objects</p>
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
<h3 id="notification.fluxcd.io/v1alpha1.AlertStatus">AlertStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.fluxcd.io/v1alpha1.Alert">Alert</a>)
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
<a href="#notification.fluxcd.io/v1alpha1.Condition">
[]Condition
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
<h3 id="notification.fluxcd.io/v1alpha1.Condition">Condition
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.fluxcd.io/v1alpha1.AlertStatus">AlertStatus</a>, 
<a href="#notification.fluxcd.io/v1alpha1.ProviderStatus">ProviderStatus</a>)
</p>
<p>Condition contains condition information for a notification object.</p>
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
<p>Type of the condition, currently (&lsquo;Ready&rsquo;).</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Status of the condition, one of (&lsquo;True&rsquo;, &lsquo;False&rsquo;, &lsquo;Unknown&rsquo;).</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>LastTransitionTime is the timestamp corresponding to the last status
change of this condition.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br>
<em>
string
</em>
</td>
<td>
<p>Reason is a brief machine readable explanation for the condition&rsquo;s last
transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Message is a human readable description of the details of the last
transition, complementing reason.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.fluxcd.io/v1alpha1.CrossNamespaceObjectReference">CrossNamespaceObjectReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.fluxcd.io/v1alpha1.AlertSpec">AlertSpec</a>)
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
<h3 id="notification.fluxcd.io/v1alpha1.ProviderSpec">ProviderSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.fluxcd.io/v1alpha1.Provider">Provider</a>)
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
<p>HTTP(S) webhook address of this provider</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Secret reference containing the provider webhook URL</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="notification.fluxcd.io/v1alpha1.ProviderStatus">ProviderStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#notification.fluxcd.io/v1alpha1.Provider">Provider</a>)
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
<code>conditions</code><br>
<em>
<a href="#notification.fluxcd.io/v1alpha1.Condition">
[]Condition
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
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
