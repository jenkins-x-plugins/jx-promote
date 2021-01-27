---
title: API Documentation
linktitle: API Documentation
description: Reference of the jx-promote configuration
weight: 10
---
<p>Packages:</p>
<ul>
<li>
<a href="#promote.jenkins-x.io%2fv1alpha1">promote.jenkins-x.io/v1alpha1</a>
</li>
</ul>
<h2 id="promote.jenkins-x.io/v1alpha1">promote.jenkins-x.io/v1alpha1</h2>
<p>
<p>Package v1alpha1 is the v1alpha1 version of the API.</p>
</p>
Resource Types:
<ul><li>
<a href="#promote.jenkins-x.io/v1alpha1.Promote">Promote</a>
</li></ul>
<h3 id="promote.jenkins-x.io/v1alpha1.Promote">Promote
</h3>
<p>
<p>Promote represents the boot configuration</p>
</p>
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
<code>apiVersion</code></br>
string</td>
<td>
<code>
promote.jenkins-x.io/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>Promote</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
<em>(Optional)</em>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.PromoteSpec">
PromoteSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Spec holds the boot configuration</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>fileRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.FileRule">
FileRule
</a>
</em>
</td>
<td>
<p>File specifies a promotion rule for a File such as for a Makefile or shell script</p>
</td>
</tr>
<tr>
<td>
<code>helmRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.HelmRule">
HelmRule
</a>
</em>
</td>
<td>
<p>HelmRule specifies a composite helm chart to promote to by adding the app to the charts
&lsquo;requirements.yaml&rsquo; file</p>
</td>
</tr>
<tr>
<td>
<code>helmfileRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.HelmfileRule">
HelmfileRule
</a>
</em>
</td>
<td>
<p>HelmfileRule specifies the location of the helmfile to promote into</p>
</td>
</tr>
<tr>
<td>
<code>kptRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.KptRule">
KptRule
</a>
</em>
</td>
<td>
<p>KptRule specifies to fetch the apps resource via kpt : <a href="https://googlecontainertools.github.io/kpt/">https://googlecontainertools.github.io/kpt/</a></p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="promote.jenkins-x.io/v1alpha1.FileRule">FileRule
</h3>
<p>
(<em>Appears on:</em>
<a href="#promote.jenkins-x.io/v1alpha1.PromoteSpec">PromoteSpec</a>)
</p>
<p>
<p>FileRule specifies how to modify a &lsquo;Makefile` or shell script to add a new helm/kpt style command</p>
</p>
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
<code>path</code></br>
<em>
string
</em>
</td>
<td>
<p>Path the path to the Makefile or shell script to modify. This is mandatory</p>
</td>
</tr>
<tr>
<td>
<code>linePrefix</code></br>
<em>
string
</em>
</td>
<td>
<p>LinePrefix adds a prefix to lines. e.g. for a Makefile that is typically &ldquo;\t&rdquo;</p>
</td>
</tr>
<tr>
<td>
<code>insertAfter</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.LineMatcher">
[]LineMatcher
</a>
</em>
</td>
<td>
<p>InsertAfter finds the last line to match against to find where to insert</p>
</td>
</tr>
<tr>
<td>
<code>updateTemplate</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.LineMatcher">
LineMatcher
</a>
</em>
</td>
<td>
<p>UpdateTemplate matches line to perform upgrades to an app</p>
</td>
</tr>
<tr>
<td>
<code>commandTemplate</code></br>
<em>
string
</em>
</td>
<td>
<p>CommandTemplate the command template for the promote command</p>
</td>
</tr>
</tbody>
</table>
<h3 id="promote.jenkins-x.io/v1alpha1.HelmRule">HelmRule
</h3>
<p>
(<em>Appears on:</em>
<a href="#promote.jenkins-x.io/v1alpha1.PromoteSpec">PromoteSpec</a>)
</p>
<p>
<p>HelmRule specifies which chart to add the app to the Chart&rsquo;s &lsquo;requirements.yaml&rsquo; file</p>
</p>
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
<code>path</code></br>
<em>
string
</em>
</td>
<td>
<p>Path to the chart folder (which should contain Chart.yaml and requirements.yaml)</p>
</td>
</tr>
</tbody>
</table>
<h3 id="promote.jenkins-x.io/v1alpha1.HelmfileRule">HelmfileRule
</h3>
<p>
(<em>Appears on:</em>
<a href="#promote.jenkins-x.io/v1alpha1.PromoteSpec">PromoteSpec</a>)
</p>
<p>
<p>HelmfileRule specifies which &lsquo;helmfile.yaml&rsquo; file to use to promote the app into</p>
</p>
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
<code>path</code></br>
<em>
string
</em>
</td>
<td>
<p>Path to the helmfile to modify</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code></br>
<em>
string
</em>
</td>
<td>
<p>Namespace if specified the given namespace is used in the <code>helmfile.yml</code> file when using Environments in the
same cluster using the same git repository URL as the dev environment</p>
</td>
</tr>
<tr>
<td>
<code>KeepOldVersions</code></br>
<em>
[]string
</em>
</td>
<td>
<p>KeepOldVersions if specified will cause the named repo/release releases to be retailed in the helmfile</p>
</td>
</tr>
</tbody>
</table>
<h3 id="promote.jenkins-x.io/v1alpha1.KptRule">KptRule
</h3>
<p>
(<em>Appears on:</em>
<a href="#promote.jenkins-x.io/v1alpha1.PromoteSpec">PromoteSpec</a>)
</p>
<p>
<p>KptRule specifies to fetch the apps resource via kpt : <a href="https://googlecontainertools.github.io/kpt/">https://googlecontainertools.github.io/kpt/</a></p>
</p>
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
<code>path</code></br>
<em>
string
</em>
</td>
<td>
<p>Path specifies the folder to fetch kpt resources into.
For example if the &lsquo;config-root&rdquo; directory contains a Config Sync git layout we may want applications to be deployed into the
<code>config-root/namespaces/myapps</code> folder. If so set the path to <code>config-root/namespaces/myapps</code></p>
</td>
</tr>
<tr>
<td>
<code>namespace</code></br>
<em>
string
</em>
</td>
<td>
<p>Namespace specifies the namespace to deploy applications if using kpt. If specified this value will be used instead
of the Environment.Spec.Namespace in the Environment CRD</p>
</td>
</tr>
</tbody>
</table>
<h3 id="promote.jenkins-x.io/v1alpha1.LineMatcher">LineMatcher
</h3>
<p>
(<em>Appears on:</em>
<a href="#promote.jenkins-x.io/v1alpha1.FileRule">FileRule</a>)
</p>
<p>
<p>LineMatcher specifies a rule on how to find a line to match</p>
</p>
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
<code>prefix</code></br>
<em>
string
</em>
</td>
<td>
<p>Prefix the prefix of a line to match</p>
</td>
</tr>
<tr>
<td>
<code>regex</code></br>
<em>
string
</em>
</td>
<td>
<p>Regex the regex of a line to match</p>
</td>
</tr>
</tbody>
</table>
<h3 id="promote.jenkins-x.io/v1alpha1.PromoteSpec">PromoteSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#promote.jenkins-x.io/v1alpha1.Promote">Promote</a>)
</p>
<p>
<p>PromoteSpec defines the desired state of Promote.</p>
</p>
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
<code>fileRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.FileRule">
FileRule
</a>
</em>
</td>
<td>
<p>File specifies a promotion rule for a File such as for a Makefile or shell script</p>
</td>
</tr>
<tr>
<td>
<code>helmRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.HelmRule">
HelmRule
</a>
</em>
</td>
<td>
<p>HelmRule specifies a composite helm chart to promote to by adding the app to the charts
&lsquo;requirements.yaml&rsquo; file</p>
</td>
</tr>
<tr>
<td>
<code>helmfileRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.HelmfileRule">
HelmfileRule
</a>
</em>
</td>
<td>
<p>HelmfileRule specifies the location of the helmfile to promote into</p>
</td>
</tr>
<tr>
<td>
<code>kptRule</code></br>
<em>
<a href="#promote.jenkins-x.io/v1alpha1.KptRule">
KptRule
</a>
</em>
</td>
<td>
<p>KptRule specifies to fetch the apps resource via kpt : <a href="https://googlecontainertools.github.io/kpt/">https://googlecontainertools.github.io/kpt/</a></p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>8404900</code>.
</em></p>
