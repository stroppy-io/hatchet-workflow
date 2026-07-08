

<a name="cloud-v1-iam"></a>
# cloud.v1.iam

## Table of Contents
- Messages
  - [cloud.v1.iam.Account](#cloud-v1-iam-account)
  - [cloud.v1.iam.Action](#cloud-v1-iam-action)
  - [cloud.v1.iam.ApiToken](#cloud-v1-iam-apitoken)
  - [cloud.v1.iam.ApiTokenType](#cloud-v1-iam-apitokentype)
  - [cloud.v1.iam.ExternalIdentity](#cloud-v1-iam-externalidentity)
  - [cloud.v1.iam.IdentityProvider](#cloud-v1-iam-identityprovider)
  - [cloud.v1.iam.Membership](#cloud-v1-iam-membership)
  - [cloud.v1.iam.Permission](#cloud-v1-iam-permission)
  - [cloud.v1.iam.Resource](#cloud-v1-iam-resource)
  - [cloud.v1.iam.Role](#cloud-v1-iam-role)
  - [cloud.v1.iam.Scope](#cloud-v1-iam-scope)
  - [cloud.v1.iam.Tenant](#cloud-v1-iam-tenant)

<a name="cloud-v1-iam-messages"></a>
## Messages

<a name="cloud-v1-iam-account"></a>
### cloud.v1.iam.Account

<pre>
//Account is a global principal (a human or a machine identity).

//Authentication secrets are intentionally ABSENT: no password hash, no
//tokens, no API keys live on this message. Those belong to the auth
//subsystem and are stored/transported separately so the identity model can
//be passed around (logs, list responses, membership joins) without leaking
//credentials.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//created_at is when the account was first created (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>email</td>
<td>string</td>
<td><pre>
//email is the account's contact + login address. Format-validated and
//expected to be unique across the control plane.<br>

json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>email_verified</td>
<td>bool</td>
<td><pre>
//email_verified is true once the address has been confirmed via the
//verification flow (IamAPI.VerifyEmail). A freshly registered account
//starts false; SSO-provisioned and admin-created accounts MAY start true.
//It is informational — login is NOT blocked on it (see RegisterResponse).<br>

json_name: emailVerified
go_name: EmailVerified</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable, server-assigned unique identifier (e.g. UUID).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>is_admin</td>
<td>bool</td>
<td><pre>
//is_admin is the platform super-user flag. When true the account holds
//full authority across every tenant REGARDLESS of any Membership — the
//break-glass escape hatch. Keep the set of such accounts tiny.<br>

json_name: isAdmin
go_name: IsAdmin</pre></td>
</tr><tr>
<td>nickname</td>
<td>string</td>
<td><pre>
//nickname is the human-friendly handle shown in UIs and usable as an
//alternate login. Restricted to URL/handle-safe characters.<br>

json_name: nickname
go_name: Nickname</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//updated_at is when the account was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-iam-action"></a>
### cloud.v1.iam.Action

<pre>
//Action enumerates the verbs a Permission can allow on its Resource. The
//first five mirror standard CRUD+list. ACTION_MANAGE is a deliberate wildcard
//so a role can say "everything on this resource" without listing each verb;
//the authorization layer MUST treat ACTION_MANAGE as implying every other
//action on the same Resource.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>ACTION_UNSPECIFIED</td>
<td><pre>
//ACTION_UNSPECIFIED is the zero value and is never a valid grant.
</pre></td>
</tr><tr>
<td>ACTION_CREATE</td>
<td><pre>
//ACTION_CREATE allows creating new instances of the resource.
</pre></td>
</tr><tr>
<td>ACTION_READ</td>
<td><pre>
//ACTION_READ allows fetching a single instance by id.
</pre></td>
</tr><tr>
<td>ACTION_UPDATE</td>
<td><pre>
//ACTION_UPDATE allows mutating an existing instance.
</pre></td>
</tr><tr>
<td>ACTION_DELETE</td>
<td><pre>
//ACTION_DELETE allows removing an instance.
</pre></td>
</tr><tr>
<td>ACTION_LIST</td>
<td><pre>
//ACTION_LIST allows enumerating/searching instances.
</pre></td>
</tr><tr>
<td>ACTION_MANAGE</td>
<td><pre>
//ACTION_MANAGE is the wildcard verb: implies every other action above.
</pre></td>
</tr>
</table>

<a name="cloud-v1-iam-apitoken"></a>
### cloud.v1.iam.ApiToken

<pre>
//ApiToken is the metadata of one programmatic credential. The secret itself is
//NOT here (write-only, returned once on create); this message is safe to list,
//log and show.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account_id</td>
<td>string</td>
<td><pre>
//account_id references the Account this token authenticates as.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr><tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//created_at is when the token was minted (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>expires_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//expires_at is when the token stops being accepted. Unset -> no expiry.<br>

json_name: expiresAt
go_name: ExpiresAt</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable, server-assigned unique identifier (e.g. UUID).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>last_used_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//last_used_at is the server's best-effort record of the most recent
//successful authentication with this token. Unset -> never used.<br>

json_name: lastUsedAt
go_name: LastUsedAt</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is a human label so an operator can tell tokens apart, e.g.
//"ci-deploy" or "laptop".<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>permissions</td>
<td><a href="#cloud-v1-iam-permission">cloud.v1.iam.Permission</a></td>
<td><pre>
//permissions is the granted subset for a SERVICE token; it MUST be empty
//for a PERSONAL token (which inherits the account's authority instead). The
//effective authority is still capped by the owner's live permissions — this
//set only narrows, never widens.<br>

json_name: permissions
go_name: Permissions</pre></td>
</tr><tr>
<td>prefix</td>
<td>string</td>
<td><pre>
//prefix is the non-secret, indexed identifier embedded in the token string
//(the part before the secret), shown in UIs to identify a token without
//revealing it, e.g. "stp_AbC123".<br>

json_name: prefix
go_name: Prefix</pre></td>
</tr><tr>
<td>type</td>
<td><a href="#cloud-v1-iam-apitokentype">cloud.v1.iam.ApiTokenType</a></td>
<td><pre>
//type selects the authority model (see ApiTokenType).<br>

json_name: type
go_name: Type</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//updated_at is when the token was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-iam-apitokentype"></a>
### cloud.v1.iam.ApiTokenType

<pre>
//ApiTokenType distinguishes the two archetypes, which differ ONLY in how their
//effective permissions are computed:

//- API_TOKEN_TYPE_PERSONAL: the token acts as its owning Account. Its
//authority is the account's full, live authority resolved per request
//(membership roles, and is_admin if the owner is a platform admin) — the
//same as a session. Treat its secret like the owner's password. Intended
//for a human's own scripts.

//- API_TOKEN_TYPE_SERVICE: a restricted machine/CI credential. It carries an
//explicit permission subset (permissions) and its effective authority is
//the INTERSECTION of that subset with the owner's live authority in the
//active tenant — a token can never exceed its owner, and a SERVICE token is
//NEVER is_admin. A leaked CI token is therefore bounded.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>API_TOKEN_TYPE_UNSPECIFIED</td>
<td><pre>
//API_TOKEN_TYPE_UNSPECIFIED is the zero value and is never a valid type.
</pre></td>
</tr><tr>
<td>API_TOKEN_TYPE_PERSONAL</td>
<td><pre>
//API_TOKEN_TYPE_PERSONAL acts as the owning account (inherits live authority).
</pre></td>
</tr><tr>
<td>API_TOKEN_TYPE_SERVICE</td>
<td><pre>
//API_TOKEN_TYPE_SERVICE is a CI/machine token capped to a permission subset.
</pre></td>
</tr>
</table>

<a name="cloud-v1-iam-externalidentity"></a>
### cloud.v1.iam.ExternalIdentity

<pre>
//ExternalIdentity links one IdP subject to one local Account. The pair
//(provider_id, subject) is unique and is what a callback resolves to find the
//Account to log in.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account_id</td>
<td>string</td>
<td><pre>
//account_id references the local Account this identity is linked to.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr><tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//created_at is when the link was first created (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>email</td>
<td>string</td>
<td><pre>
//email is the address asserted by the IdP at link time, kept for display
//and domain checks; it is NOT authority (the subject is).<br>

json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable, server-assigned unique identifier (e.g. UUID).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>provider_id</td>
<td>string</td>
<td><pre>
//provider_id references the IdentityProvider this identity came from.<br>

json_name: providerId
go_name: ProviderId</pre></td>
</tr><tr>
<td>subject</td>
<td>string</td>
<td><pre>
//subject is the IdP's stable subject identifier (the OIDC "sub" claim),
//unique within the provider and immutable for the user's lifetime there.<br>

json_name: subject
go_name: Subject</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//updated_at is when the link was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-iam-identityprovider"></a>
### cloud.v1.iam.IdentityProvider

<pre>
//IdentityProvider is one configured OIDC provider. The client_secret is NOT a
//field here: like a password it is write-only, supplied on create/update and
//stored server-side, never read back.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>allowed_domains</td>
<td>string</td>
<td><pre>
//allowed_domains restricts which email domains may sign in / be
//provisioned through this provider, e.g. ["acme.com"]. Empty -> any domain
//the IdP returns is accepted (only safe when auto_provision is false; see
//auto_provision).<br>

json_name: allowedDomains
go_name: AllowedDomains</pre></td>
</tr><tr>
<td>auto_provision</td>
<td>bool</td>
<td><pre>
//auto_provision enables JIT account creation: on a first SSO login whose
//email domain is allowed, the server creates and links a new Account.
//When false, only a pre-linked Account can log in through this provider.

//SECURITY: auto_provision REQUIRES a non-empty allowed_domains. Otherwise
//anyone with an account at the IdP could mint a local Account, silently
//bypassing PlatformSettings.allow_self_registration. The service layer
//rejects auto_provision == true with empty allowed_domains.<br>

json_name: autoProvision
go_name: AutoProvision</pre></td>
</tr><tr>
<td>client_id</td>
<td>string</td>
<td><pre>
//client_id is the OAuth2 client identifier registered at the IdP. Public
//by OAuth2 design, so it is safe to expose; the secret is not.<br>

json_name: clientId
go_name: ClientId</pre></td>
</tr><tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//created_at is when the provider was first configured (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>disabled</td>
<td>bool</td>
<td><pre>
//disabled hides the provider without deleting it: a disabled provider
//shows no login button and rejects its callback. Polarity is inverted on
//purpose so the zero value (false) means ENABLED — a freshly created
//provider works immediately, no follow-up edit needed.<br>

json_name: disabled
go_name: Disabled</pre></td>
</tr><tr>
<td>display_name</td>
<td>string</td>
<td><pre>
//display_name is the human label shown on the login button, e.g. "Google".<br>

json_name: displayName
go_name: DisplayName</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable, server-assigned unique identifier (e.g. UUID).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>issuer</td>
<td>string</td>
<td><pre>
//issuer is the OIDC issuer / discovery base URL, e.g.
//https://accounts.google.com. The server fetches
//<issuer>/.well-known/openid-configuration to learn the auth/token/jwks
//endpoints. MUST be https — token/jwks fetched over it are trust-critical.<br>

json_name: issuer
go_name: Issuer</pre></td>
</tr><tr>
<td>scopes</td>
<td>string</td>
<td><pre>
//scopes requested at the IdP. Empty -> the server defaults to
//["openid", "email", "profile"].<br>

json_name: scopes
go_name: Scopes</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
//slug is the url-safe handle used in the SSO routes:
///auth/sso/<slug>/start and /auth/sso/<slug>/callback. Unique.<br>

json_name: slug
go_name: Slug</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//updated_at is when the provider was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-iam-membership"></a>
### cloud.v1.iam.Membership

<pre>
//membership.proto wires the model together. A Membership is the join that
//grants an Account access inside a Tenant by attaching one or more Roles. It
//is the ONLY path to tenant access for a non-admin account: no Membership,
//no entry.

//Effective authorization for a caller in a tenant = the union of the
//Permissions of every Role referenced here (plus any SCOPE_PLATFORM roles the
//account holds independently, and the full-access shortcut when
//Account.is_admin is true).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account_id</td>
<td>string</td>
<td><pre>
//account_id references the Account being granted access.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr><tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//created_at is when the membership was first created (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable, server-assigned unique identifier (e.g. UUID).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>role_ids</td>
<td>string</td>
<td><pre>
//role_ids references Role.id values granted by this membership. At least
//one role is required — a membership with no roles grants nothing and
//should not exist. The referenced roles are expected to be either
//SCOPE_TENANT roles of this same tenant_id or SCOPE_PLATFORM roles;
//cross-tenant role references are rejected by the service layer.<br>

json_name: roleIds
go_name: RoleIds</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id references the Tenant the access is scoped to.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//updated_at is when the membership was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-iam-permission"></a>
### cloud.v1.iam.Permission

<pre>
//Permission is one (resource, action) capability — the smallest unit of
//authority in the system. Permissions are never granted directly to an
//Account; they are bundled into a Role, and Roles are granted via Membership.
//Both fields reject their zero value: a permission with an unspecified
//resource or action is meaningless and must not be storable.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>action</td>
<td><a href="#cloud-v1-iam-action">cloud.v1.iam.Action</a></td>
<td><pre>
//action is the verb this capability allows on resource.<br>

json_name: action
go_name: Action</pre></td>
</tr><tr>
<td>resource</td>
<td><a href="#cloud-v1-iam-resource">cloud.v1.iam.Resource</a></td>
<td><pre>
//resource is the object class this capability acts on.<br>

json_name: resource
go_name: Resource</pre></td>
</tr>
</table>



<a name="cloud-v1-iam-resource"></a>
### cloud.v1.iam.Resource

<pre>
//Resource enumerates the object classes the control plane protects. Every
//Permission names exactly one Resource. Add a new value here the moment a new
//protected object class appears — an unlisted resource can never be granted,
//which is the safe default.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>RESOURCE_UNSPECIFIED</td>
<td><pre>
//RESOURCE_UNSPECIFIED is the zero value and is never a valid grant.
</pre></td>
</tr><tr>
<td>RESOURCE_ACCOUNT</td>
<td><pre>
//RESOURCE_ACCOUNT is the global identity object (read/list/update/delete
//accounts; creation is admin-only, not granted via this resource).
</pre></td>
</tr><tr>
<td>RESOURCE_TENANT</td>
<td><pre>
//RESOURCE_TENANT is the workspace object itself (rename, delete, transfer).
</pre></td>
</tr><tr>
<td>RESOURCE_ROLE</td>
<td><pre>
//RESOURCE_ROLE covers role definitions and the permission sets they carry.
</pre></td>
</tr><tr>
<td>RESOURCE_MEMBERSHIP</td>
<td><pre>
//RESOURCE_MEMBERSHIP covers the account<->tenant role bindings.
</pre></td>
</tr><tr>
<td>RESOURCE_SETTINGS</td>
<td><pre>
//RESOURCE_SETTINGS is the control-plane (platform) configuration.
</pre></td>
</tr><tr>
<td>RESOURCE_PRESET</td>
<td><pre>
//RESOURCE_PRESET is a tenant-scoped, reusable test/database/workload
//preset (see models.DatabasePreset / WorkloadPreset / TestPreset).
</pre></td>
</tr><tr>
<td>RESOURCE_WIZARD</td>
<td><pre>
//RESOURCE_WIZARD is a tenant-scoped wizard draft (see
//models.TestWizardDraftRecord / SuiteWizardDraftRecord) used to assemble a
//TestRun / SuiteRun step by step.
</pre></td>
</tr><tr>
<td>RESOURCE_TEST_RUN</td>
<td><pre>
//RESOURCE_TEST_RUN is a tenant-scoped test execution (models.TestRunRecord):
//start, read, list, cancel, delete.
</pre></td>
</tr><tr>
<td>RESOURCE_SUITE</td>
<td><pre>
//RESOURCE_SUITE is a tenant-scoped suite definition (models.SuiteRecord):
//full CRUD + clone.
</pre></td>
</tr><tr>
<td>RESOURCE_SUITE_RUN</td>
<td><pre>
//RESOURCE_SUITE_RUN is a tenant-scoped suite execution (models.SuiteRunRecord):
//start, read, list, cancel, delete.
</pre></td>
</tr><tr>
<td>RESOURCE_FAVORITE</td>
<td><pre>
//RESOURCE_FAVORITE is the per-user favorite relation (models.FavoriteRecord).
//Tenant-scoped so favoriting requires tenant membership; favorites are
//personal to the caller. Reading the computed Entity.is_favorite flag needs
//no grant (it rides on data the caller already reads).
</pre></td>
</tr><tr>
<td>RESOURCE_AGENT_SHELL</td>
<td><pre>
//RESOURCE_AGENT_SHELL is the interactive reverse-shell to an agent from the
//admin UI. High-privilege (arbitrary command execution on the host): grant
//sparingly, audit every session.
</pre></td>
</tr><tr>
<td>RESOURCE_SHARE</td>
<td><pre>
//RESOURCE_SHARE manages public share links for a run (models.ShareRecord):
//create / list / revoke. The PUBLIC resolve endpoint is unauthenticated and
//needs no grant.
</pre></td>
</tr><tr>
<td>RESOURCE_PACKAGE</td>
<td><pre>
//RESOURCE_PACKAGE is a tenant-uploaded custom package (models.PackageRecord),
//e.g. a custom .deb / binary used to install a database build. Tenant-private.
</pre></td>
</tr><tr>
<td>RESOURCE_RECIPE</td>
<td><pre>
//RESOURCE_RECIPE is a tenant-scoped persisted DSL recipe bundle
//(models.RecipeRecord): full CRUD + check.
</pre></td>
</tr><tr>
<td>RESOURCE_PROVIDER</td>
<td><pre>
//RESOURCE_PROVIDER is a tenant-scoped catalog provider definition (see
//SP-B CatalogService): the deployment-target templates a workflow can
//target.
</pre></td>
</tr><tr>
<td>RESOURCE_WORKFLOW</td>
<td><pre>
//RESOURCE_WORKFLOW is a tenant-scoped catalog workflow definition (see
//SP-B CatalogService): the reusable DSL workflow templates a recipe can
//be built from.
</pre></td>
</tr>
</table>

<a name="cloud-v1-iam-role"></a>
### cloud.v1.iam.Role

<pre>
//Role is a named, scoped collection of Permissions.

//Scope decides reach:
//- SCOPE_PLATFORM: tenant_id MUST be empty; the role applies across the
//whole control plane (e.g. a platform auditor).
//- SCOPE_TENANT: tenant_id MUST be set; the role only grants inside that
//one tenant. The same logical role ("admin") therefore exists once per
//tenant as distinct rows.

//is_system marks built-in roles (owner/admin/viewer and the like) that the
//server seeds and that clients may read but never edit or delete. Custom
//roles created by operators leave is_system false and are fully mutable.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//created_at is when the role was first created (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable, server-assigned unique identifier (e.g. UUID).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>is_system</td>
<td>bool</td>
<td><pre>
//is_system marks built-in, immutable roles seeded by the server.<br>

json_name: isSystem
go_name: IsSystem</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the human-readable label, unique within its scope/tenant.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>permissions</td>
<td><a href="#cloud-v1-iam-permission">cloud.v1.iam.Permission</a></td>
<td><pre>
//permissions is the set of capabilities this role grants. An empty set is
//a valid (if useless) role; duplicates are harmless and ignored by the
//authorization layer, which unions all permissions anyway.<br>

json_name: permissions
go_name: Permissions</pre></td>
</tr><tr>
<td>scope</td>
<td><a href="#cloud-v1-iam-scope">cloud.v1.iam.Scope</a></td>
<td><pre>
//scope decides whether this role is platform-wide or tenant-local.<br>

json_name: scope
go_name: Scope</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id binds a SCOPE_TENANT role to its tenant. It MUST be empty for
//SCOPE_PLATFORM roles. The scope<->tenant_id consistency is enforced in
//the service layer (proto cannot express the conditional requirement).<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//updated_at is when the role was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-iam-scope"></a>
### cloud.v1.iam.Scope

<pre>
//Scope bounds where a Role (and therefore its Permissions) takes effect.
//A platform-scoped grant ignores tenant boundaries; a tenant-scoped grant is
//only meaningful inside the one tenant the role is bound to.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>SCOPE_UNSPECIFIED</td>
<td><pre>
//SCOPE_UNSPECIFIED is the zero value and is never a valid scope.
</pre></td>
</tr><tr>
<td>SCOPE_PLATFORM</td>
<td><pre>
//SCOPE_PLATFORM applies control-plane-wide, across every tenant.
</pre></td>
</tr><tr>
<td>SCOPE_TENANT</td>
<td><pre>
//SCOPE_TENANT applies only within the tenant the role is bound in.
</pre></td>
</tr>
</table>

<a name="cloud-v1-iam-tenant"></a>
### cloud.v1.iam.Tenant

<pre>
//Tenant is an isolated workspace.

//It is created and owned by an Account (owner_account_id). The owner is a
//pointer only — the actual access an owner enjoys is still expressed through
//a Membership + Role binding, so ownership and authorization stay decoupled
//and auditable. Deleting the owning account does not implicitly delete the
//tenant; that lifecycle decision lives in the service layer.

//IDENTITY vs ROUTING. A tenant has TWO names with different jobs:
//- id   — the canonical, immutable, server-assigned key. This is the ONLY
//thing authorization and storage ever key on.
//- slug — the url-safe handle shown to users in routes: /t/<slug>/...
//Human-typeable, mutable, unique. URLs use slug for readability;
//the server resolves slug -> id at the tenant gate on each request.
//- name — the free-form display label (any characters, spaces) for UIs.

//The access token is TENANT-AGNOSTIC (see claims.proto): it carries no tenant
//at all. The active tenant comes from the URL slug, is resolved to id, and the
//account's Membership in that id decides access — per request.

//NEVER authorize on slug or name: both are mutable and reassignable, so
//trusting them would let a rename silently transfer access. Resolve to id
//first, then check the account's Membership/permissions against the id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//created_at is when the tenant was first created (server clock).<br>

json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the stable, server-assigned unique identifier (e.g. UUID).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the human-readable display label shown in tenant switchers.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>owner_account_id</td>
<td>string</td>
<td><pre>
//owner_account_id references the Account that owns this tenant. It is a
//soft pointer for attribution/UI; effective permissions still come from
//Membership, not from this field.<br>

json_name: ownerAccountId
go_name: OwnerAccountId</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
//slug is the url-safe, unique handle used in user-facing routes:
///t/<slug>/runs, /t/<slug>/settings, ... It is what end users see and
//type, so it is restricted to lowercase letters, digits, '-' and '_'.

//slug is MUTABLE: an operator may rename it, but doing so breaks every
//existing link to the old slug (the service layer should 301-redirect old
//-> new where it can). Because it can change and be reassigned, slug is
//NEVER used for authorization or stored as a foreign key — id is the
//canonical reference. The tenant gate maps the URL slug to id once per
//request, then checks the account's Membership against that id.<br>

json_name: slug
go_name: Slug</pre></td>
</tr><tr>
<td>tags</td>
<td><a href="../common/README.md#cloud-v1-common-tags">cloud.v1.common.Tags</a></td>
<td><pre>
//tags is a free label/key-value set for filtering and grouping.<br>

json_name: tags
go_name: Tags</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//updated_at is when the tenant was last mutated (server clock).<br>

json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>

