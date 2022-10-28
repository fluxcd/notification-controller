# Git Commit Status Updates

The notification-controller can mark Git commits as reconciled by posting
Flux `Kustomization` events to the origin repository using Git SaaS providers APIs.

## Example

The following is an example of how to update the Git commit status for the GitHub repository where
Flux was bootstrapped with `flux bootstrap github --owner=my-gh-org --repository=my-gh-repo`.

```yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: github-status
  namespace: flux-system
spec:
  type: github
  address: https://github.com/my-gh-org/my-gh-repo
  secretRef:
    name: github-token
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Alert
metadata:
  name: github-status
  namespace: flux-system
spec:
  providerRef:
    name: github-status
  eventSources:
    - kind: Kustomization
      name: flux-system
```

In the above example:

- A Provider named `github-status` is created, indicated by the
  `Provider.metadata.name` field.
- An Alert named `github-status` is created, indicated by the
  `Alert.metadata.name` field.
- The Alert references the GitHub provider, indicated by the
  `Alert.spec.providerRef` field.
- The notification-controller starts listening for events sent by
  the `flux-system` Kustomization.
- When an event is received, the controller extracts the Git commit SHA
  from the [event](events.md) payload.
- The controller uses the GitHub PAT from the secret indicated by the
  `Provider.spec.secretRef.name` to authenticate with the GitHub API.
- The controller sets the commit status to `kustomization/flux-system/<UID>`
  followed by the success or error message from the [event](events.md) body.


## Writing a Git commit status provider spec

As with all other Kubernetes config, a Provider needs `apiVersion`,
`kind`, and `metadata` fields. The name of an Alert object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).
A Provider also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Type

`.spec.type` is a required field that specifies which Git SaaS vendor hosts the repository.

The supported types are: `github`, `gitlab`, `bitbucket` and `azuredevops`.

### Address

`.spec.address` is a required field that specifies the HTTPS URL of the Git repository
where the commits originate from.

### Secret reference

`.spec.secretRef.name` is a required field to specify a name reference to a
Secret in the same namespace as the Provider, containing the authentication
credentials for the Git SaaS API.

#### GitHub authentication

When `.spec.type` is set to `github`, the referenced secret must contain a key called `token` with the value set to a
[GitHub personal access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token).

The token must has permissions to update the commit status for the GitHub repository specified in `.spec.address`. 

You can create the secret with `kubectl` like this:

```shell
kubectl create secret generic github-token --from-literal=token=<GITHUB-TOKEN>
```

#### GitLab authentication

When `.spec.type` is set to `gitlab`, the referenced secret must contain a key called `token` with the value set to a
[GitLab personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html).

The token must has permissions to update the commit status for the GitLab repository specified in `.spec.address`.

You can create the secret with `kubectl` like this:

```shell
kubectl create secret generic gitlab-token --from-literal=token=<GITLAB-TOKEN>
```

#### BitBucket authentication

When `.spec.type` is set to `bitbucket`, the referenced secret must contain a key called `token` with the value
set to a BitBucket username and an
[app password](https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/#Create-an-app-password)
in the format `<username>:<app-password>`.

The app password must have `Repositories (Read/Write)` permission for
the BitBucket repository specified in `.spec.address`.

You can create the secret with `kubectl` like this:

```shell
kubectl create secret generic gitlab-token --from-literal=token=<username>:<app-password>
```

#### Azure DevOps authentication

When `.spec.type` is set to `azuredevops`, the referenced secret must contain a key called `token` with the value set to a
[Azure DevOps personal access token](https://docs.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops&tabs=preview-page).

The token must has permissions to update the commit status for the Azure DevOps repository specified in `.spec.address`.

You can create the secret with `kubectl` like this:

```shell
kubectl create secret generic github-token --from-literal=token=<AZURE-TOKEN>
```

### Suspend

`.spec.suspend` is an optional field to suspend the Git commit status updates.
When set to `true`, the controller will stop processing events for this provider.
When the field is set to `false` or removed, it will resume the commit status updates.
