# LangfuseOrganization

`langfuseorganizations.langfuse.palena.ai/v1alpha1`

Manages a Langfuse organization via the Admin API.

## Spec

| Field | Type | Description |
|---|---|---|
| `instanceRef` | ObjectReference | Reference to the parent `LangfuseInstance` |
| `displayName` | string | Organization name in Langfuse |
| `members` | OrganizationMembersSpec | Membership configuration |

### OrganizationMembersSpec

| Field | Type | Default | Description |
|---|---|---|---|
| `managedExclusively` | bool | `false` | If true, removes unlisted users |
| `users` | []OrganizationMemberSpec | | List of members |

### OrganizationMemberSpec

| Field | Type | Description |
|---|---|---|
| `email` | string | User's email address |
| `role` | string | `owner`, `admin`, `member`, or `viewer` |

## Status

| Field | Type | Description |
|---|---|---|
| `ready` | bool | Whether the organization is fully synced |
| `organizationId` | string | Langfuse internal organization ID |
| `memberCount` | int | Total number of members |
| `syncedMembers` | int | Number of confirmed synced members |
| `projectCount` | int | Number of `LangfuseProject` CRs referencing this org |
| `conditions` | []Condition | Standard Kubernetes conditions |

### Conditions

| Type | Description |
|---|---|
| `Ready` | Organization exists and is synced |
| `Synced` | Spec matches Langfuse state |
| `MembersSynced` | All members are at desired state |

## Finalizer

`langfuse.palena.ai/organization-cleanup`

On deletion:
1. Blocks if any `LangfuseProject` CRs reference this organization
2. Deletes the organization via the Admin API
3. Removes the finalizer

## Example

```yaml
apiVersion: langfuse.palena.ai/v1alpha1
kind: LangfuseOrganization
metadata:
  name: ml-platform
  namespace: langfuse
spec:
  instanceRef:
    name: production
  displayName: "ML Platform Team"
  members:
    managedExclusively: false
    users:
      - email: "alice@example.com"
        role: owner
      - email: "bob@example.com"
        role: admin
      - email: "carol@example.com"
        role: member
```

### Print Columns

```
NAME          READY   ORG ID        MEMBERS   AGE
ml-platform   true    org_abc123    3         2d
```
