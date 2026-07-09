

<a name="cloud-v1-api"></a>
# cloud.v1.api

## Table of Contents
- Messages
  - [cloud.v1.api.AddFavoriteRequest](#cloud-v1-api-addfavoriterequest)
  - [cloud.v1.api.AddFavoriteResponse](#cloud-v1-api-addfavoriteresponse)
  - [cloud.v1.api.CancelRunRequest](#cloud-v1-api-cancelrunrequest)
  - [cloud.v1.api.CancelRunResponse](#cloud-v1-api-cancelrunresponse)
  - [cloud.v1.api.CatalogEntry](#cloud-v1-api-catalogentry)
  - [cloud.v1.api.ChangePasswordRequest](#cloud-v1-api-changepasswordrequest)
  - [cloud.v1.api.ChangePasswordResponse](#cloud-v1-api-changepasswordresponse)
  - [cloud.v1.api.CheckRecipeRequest](#cloud-v1-api-checkreciperequest)
  - [cloud.v1.api.CheckRecipeResponse](#cloud-v1-api-checkreciperesponse)
  - [cloud.v1.api.CompareRunsRequest](#cloud-v1-api-comparerunsrequest)
  - [cloud.v1.api.CompareRunsResponse](#cloud-v1-api-comparerunsresponse)
  - [cloud.v1.api.CompareView](#cloud-v1-api-compareview)
  - [cloud.v1.api.CompleteSSORequest](#cloud-v1-api-completessorequest)
  - [cloud.v1.api.CompleteSSOResponse](#cloud-v1-api-completessoresponse)
  - [cloud.v1.api.CompleteUploadRequest](#cloud-v1-api-completeuploadrequest)
  - [cloud.v1.api.CompleteUploadResponse](#cloud-v1-api-completeuploadresponse)
  - [cloud.v1.api.ConfirmPasswordResetRequest](#cloud-v1-api-confirmpasswordresetrequest)
  - [cloud.v1.api.ConfirmPasswordResetResponse](#cloud-v1-api-confirmpasswordresetresponse)
  - [cloud.v1.api.CreateAccountRequest](#cloud-v1-api-createaccountrequest)
  - [cloud.v1.api.CreateAccountResponse](#cloud-v1-api-createaccountresponse)
  - [cloud.v1.api.CreateApiTokenRequest](#cloud-v1-api-createapitokenrequest)
  - [cloud.v1.api.CreateApiTokenResponse](#cloud-v1-api-createapitokenresponse)
  - [cloud.v1.api.CreateIdentityProviderRequest](#cloud-v1-api-createidentityproviderrequest)
  - [cloud.v1.api.CreateIdentityProviderResponse](#cloud-v1-api-createidentityproviderresponse)
  - [cloud.v1.api.CreateMembershipRequest](#cloud-v1-api-createmembershiprequest)
  - [cloud.v1.api.CreateMembershipResponse](#cloud-v1-api-createmembershipresponse)
  - [cloud.v1.api.CreatePackageUploadRequest](#cloud-v1-api-createpackageuploadrequest)
  - [cloud.v1.api.CreatePackageUploadResponse](#cloud-v1-api-createpackageuploadresponse)
  - [cloud.v1.api.CreateRecipeRequest](#cloud-v1-api-createreciperequest)
  - [cloud.v1.api.CreateRecipeResponse](#cloud-v1-api-createreciperesponse)
  - [cloud.v1.api.CreateRoleRequest](#cloud-v1-api-createrolerequest)
  - [cloud.v1.api.CreateRoleResponse](#cloud-v1-api-createroleresponse)
  - [cloud.v1.api.CreateShareRequest](#cloud-v1-api-createsharerequest)
  - [cloud.v1.api.CreateShareResponse](#cloud-v1-api-createshareresponse)
  - [cloud.v1.api.CreateTenantRequest](#cloud-v1-api-createtenantrequest)
  - [cloud.v1.api.CreateTenantResponse](#cloud-v1-api-createtenantresponse)
  - [cloud.v1.api.DeleteAccountRequest](#cloud-v1-api-deleteaccountrequest)
  - [cloud.v1.api.DeleteAccountResponse](#cloud-v1-api-deleteaccountresponse)
  - [cloud.v1.api.DeleteIdentityProviderRequest](#cloud-v1-api-deleteidentityproviderrequest)
  - [cloud.v1.api.DeleteIdentityProviderResponse](#cloud-v1-api-deleteidentityproviderresponse)
  - [cloud.v1.api.DeleteMembershipRequest](#cloud-v1-api-deletemembershiprequest)
  - [cloud.v1.api.DeleteMembershipResponse](#cloud-v1-api-deletemembershipresponse)
  - [cloud.v1.api.DeletePackageRequest](#cloud-v1-api-deletepackagerequest)
  - [cloud.v1.api.DeletePackageResponse](#cloud-v1-api-deletepackageresponse)
  - [cloud.v1.api.DeleteRecipeRequest](#cloud-v1-api-deletereciperequest)
  - [cloud.v1.api.DeleteRecipeResponse](#cloud-v1-api-deletereciperesponse)
  - [cloud.v1.api.DeleteRoleRequest](#cloud-v1-api-deleterolerequest)
  - [cloud.v1.api.DeleteRoleResponse](#cloud-v1-api-deleteroleresponse)
  - [cloud.v1.api.DeleteRunRequest](#cloud-v1-api-deleterunrequest)
  - [cloud.v1.api.DeleteRunResponse](#cloud-v1-api-deleterunresponse)
  - [cloud.v1.api.DeleteShareRequest](#cloud-v1-api-deletesharerequest)
  - [cloud.v1.api.DeleteShareResponse](#cloud-v1-api-deleteshareresponse)
  - [cloud.v1.api.DeleteTenantRequest](#cloud-v1-api-deletetenantrequest)
  - [cloud.v1.api.DeleteTenantResponse](#cloud-v1-api-deletetenantresponse)
  - [cloud.v1.api.ExternalIdentityLink](#cloud-v1-api-externalidentitylink)
  - [cloud.v1.api.GetAccountRequest](#cloud-v1-api-getaccountrequest)
  - [cloud.v1.api.GetAccountResponse](#cloud-v1-api-getaccountresponse)
  - [cloud.v1.api.GetIdentityProviderRequest](#cloud-v1-api-getidentityproviderrequest)
  - [cloud.v1.api.GetIdentityProviderResponse](#cloud-v1-api-getidentityproviderresponse)
  - [cloud.v1.api.GetLogFacetsRequest](#cloud-v1-api-getlogfacetsrequest)
  - [cloud.v1.api.GetLogFacetsResponse](#cloud-v1-api-getlogfacetsresponse)
  - [cloud.v1.api.GetMembershipRequest](#cloud-v1-api-getmembershiprequest)
  - [cloud.v1.api.GetMembershipResponse](#cloud-v1-api-getmembershipresponse)
  - [cloud.v1.api.GetMyAccountRequest](#cloud-v1-api-getmyaccountrequest)
  - [cloud.v1.api.GetMyAccountResponse](#cloud-v1-api-getmyaccountresponse)
  - [cloud.v1.api.GetMyPermissionsRequest](#cloud-v1-api-getmypermissionsrequest)
  - [cloud.v1.api.GetMyPermissionsResponse](#cloud-v1-api-getmypermissionsresponse)
  - [cloud.v1.api.GetPackageRequest](#cloud-v1-api-getpackagerequest)
  - [cloud.v1.api.GetPackageResponse](#cloud-v1-api-getpackageresponse)
  - [cloud.v1.api.GetPublicConfigRequest](#cloud-v1-api-getpublicconfigrequest)
  - [cloud.v1.api.GetPublicConfigResponse](#cloud-v1-api-getpublicconfigresponse)
  - [cloud.v1.api.GetPublicRatingRequest](#cloud-v1-api-getpublicratingrequest)
  - [cloud.v1.api.GetPublicRatingResponse](#cloud-v1-api-getpublicratingresponse)
  - [cloud.v1.api.GetRecipeRequest](#cloud-v1-api-getreciperequest)
  - [cloud.v1.api.GetRecipeResponse](#cloud-v1-api-getreciperesponse)
  - [cloud.v1.api.GetRoleRequest](#cloud-v1-api-getrolerequest)
  - [cloud.v1.api.GetRoleResponse](#cloud-v1-api-getroleresponse)
  - [cloud.v1.api.GetRunMetricsRequest](#cloud-v1-api-getrunmetricsrequest)
  - [cloud.v1.api.GetRunMetricsResponse](#cloud-v1-api-getrunmetricsresponse)
  - [cloud.v1.api.GetRunQuotaUsageRequest](#cloud-v1-api-getrunquotausagerequest)
  - [cloud.v1.api.GetRunQuotaUsageResponse](#cloud-v1-api-getrunquotausageresponse)
  - [cloud.v1.api.GetShareRequest](#cloud-v1-api-getsharerequest)
  - [cloud.v1.api.GetShareResponse](#cloud-v1-api-getshareresponse)
  - [cloud.v1.api.GetSharedRunRequest](#cloud-v1-api-getsharedrunrequest)
  - [cloud.v1.api.GetSharedRunResponse](#cloud-v1-api-getsharedrunresponse)
  - [cloud.v1.api.GetSystemRatingRequest](#cloud-v1-api-getsystemratingrequest)
  - [cloud.v1.api.GetSystemRatingResponse](#cloud-v1-api-getsystemratingresponse)
  - [cloud.v1.api.GetSystemSettingsRequest](#cloud-v1-api-getsystemsettingsrequest)
  - [cloud.v1.api.GetSystemSettingsResponse](#cloud-v1-api-getsystemsettingsresponse)
  - [cloud.v1.api.GetTenantDashboardRequest](#cloud-v1-api-gettenantdashboardrequest)
  - [cloud.v1.api.GetTenantDashboardResponse](#cloud-v1-api-gettenantdashboardresponse)
  - [cloud.v1.api.GetTenantRatingRequest](#cloud-v1-api-gettenantratingrequest)
  - [cloud.v1.api.GetTenantRatingResponse](#cloud-v1-api-gettenantratingresponse)
  - [cloud.v1.api.GetTenantRequest](#cloud-v1-api-gettenantrequest)
  - [cloud.v1.api.GetTenantResponse](#cloud-v1-api-gettenantresponse)
  - [cloud.v1.api.GetTenantSettingsRequest](#cloud-v1-api-gettenantsettingsrequest)
  - [cloud.v1.api.GetTenantSettingsResponse](#cloud-v1-api-gettenantsettingsresponse)
  - [cloud.v1.api.GetTestRunOverviewRequest](#cloud-v1-api-gettestrunoverviewrequest)
  - [cloud.v1.api.GetTestRunOverviewResponse](#cloud-v1-api-gettestrunoverviewresponse)
  - [cloud.v1.api.LeaveTenantRequest](#cloud-v1-api-leavetenantrequest)
  - [cloud.v1.api.LeaveTenantResponse](#cloud-v1-api-leavetenantresponse)
  - [cloud.v1.api.LinkExternalIdentityRequest](#cloud-v1-api-linkexternalidentityrequest)
  - [cloud.v1.api.LinkExternalIdentityResponse](#cloud-v1-api-linkexternalidentityresponse)
  - [cloud.v1.api.ListAccountsRequest](#cloud-v1-api-listaccountsrequest)
  - [cloud.v1.api.ListAccountsResponse](#cloud-v1-api-listaccountsresponse)
  - [cloud.v1.api.ListApiTokensRequest](#cloud-v1-api-listapitokensrequest)
  - [cloud.v1.api.ListApiTokensResponse](#cloud-v1-api-listapitokensresponse)
  - [cloud.v1.api.ListExternalIdentitiesRequest](#cloud-v1-api-listexternalidentitiesrequest)
  - [cloud.v1.api.ListExternalIdentitiesResponse](#cloud-v1-api-listexternalidentitiesresponse)
  - [cloud.v1.api.ListFavoritesRequest](#cloud-v1-api-listfavoritesrequest)
  - [cloud.v1.api.ListFavoritesResponse](#cloud-v1-api-listfavoritesresponse)
  - [cloud.v1.api.ListIdentityProvidersRequest](#cloud-v1-api-listidentityprovidersrequest)
  - [cloud.v1.api.ListIdentityProvidersResponse](#cloud-v1-api-listidentityprovidersresponse)
  - [cloud.v1.api.ListMembershipsRequest](#cloud-v1-api-listmembershipsrequest)
  - [cloud.v1.api.ListMembershipsResponse](#cloud-v1-api-listmembershipsresponse)
  - [cloud.v1.api.ListMyTenantsRequest](#cloud-v1-api-listmytenantsrequest)
  - [cloud.v1.api.ListMyTenantsResponse](#cloud-v1-api-listmytenantsresponse)
  - [cloud.v1.api.ListPackagesRequest](#cloud-v1-api-listpackagesrequest)
  - [cloud.v1.api.ListPackagesResponse](#cloud-v1-api-listpackagesresponse)
  - [cloud.v1.api.ListPermissionsRequest](#cloud-v1-api-listpermissionsrequest)
  - [cloud.v1.api.ListPermissionsResponse](#cloud-v1-api-listpermissionsresponse)
  - [cloud.v1.api.ListQuotasRequest](#cloud-v1-api-listquotasrequest)
  - [cloud.v1.api.ListQuotasResponse](#cloud-v1-api-listquotasresponse)
  - [cloud.v1.api.ListRecipesRequest](#cloud-v1-api-listrecipesrequest)
  - [cloud.v1.api.ListRecipesResponse](#cloud-v1-api-listrecipesresponse)
  - [cloud.v1.api.ListRegistrationRequestsRequest](#cloud-v1-api-listregistrationrequestsrequest)
  - [cloud.v1.api.ListRegistrationRequestsResponse](#cloud-v1-api-listregistrationrequestsresponse)
  - [cloud.v1.api.ListRolesRequest](#cloud-v1-api-listrolesrequest)
  - [cloud.v1.api.ListRolesResponse](#cloud-v1-api-listrolesresponse)
  - [cloud.v1.api.ListRunsRequest](#cloud-v1-api-listrunsrequest)
  - [cloud.v1.api.ListRunsResponse](#cloud-v1-api-listrunsresponse)
  - [cloud.v1.api.ListSharesRequest](#cloud-v1-api-listsharesrequest)
  - [cloud.v1.api.ListSharesResponse](#cloud-v1-api-listsharesresponse)
  - [cloud.v1.api.ListStroppyVersionsRequest](#cloud-v1-api-liststroppyversionsrequest)
  - [cloud.v1.api.ListStroppyVersionsResponse](#cloud-v1-api-liststroppyversionsresponse)
  - [cloud.v1.api.LogFacetField](#cloud-v1-api-logfacetfield)
  - [cloud.v1.api.LogFacetValue](#cloud-v1-api-logfacetvalue)
  - [cloud.v1.api.LogFilter](#cloud-v1-api-logfilter)
  - [cloud.v1.api.LogScrollDirection](#cloud-v1-api-logscrolldirection)
  - [cloud.v1.api.LoginRequest](#cloud-v1-api-loginrequest)
  - [cloud.v1.api.LoginResponse](#cloud-v1-api-loginresponse)
  - [cloud.v1.api.LogoutRequest](#cloud-v1-api-logoutrequest)
  - [cloud.v1.api.LogoutResponse](#cloud-v1-api-logoutresponse)
  - [cloud.v1.api.LookupAccountByEmailRequest](#cloud-v1-api-lookupaccountbyemailrequest)
  - [cloud.v1.api.LookupAccountByEmailResponse](#cloud-v1-api-lookupaccountbyemailresponse)
  - [cloud.v1.api.MarkRegistrationRequestHandledRequest](#cloud-v1-api-markregistrationrequesthandledrequest)
  - [cloud.v1.api.MarkRegistrationRequestHandledResponse](#cloud-v1-api-markregistrationrequesthandledresponse)
  - [cloud.v1.api.PlatformSettings](#cloud-v1-api-platformsettings)
  - [cloud.v1.api.PublicRatingEntry](#cloud-v1-api-publicratingentry)
  - [cloud.v1.api.QueryLogsRequest](#cloud-v1-api-querylogsrequest)
  - [cloud.v1.api.QueryLogsResponse](#cloud-v1-api-querylogsresponse)
  - [cloud.v1.api.QuotaRefreshPolicy](#cloud-v1-api-quotarefreshpolicy)
  - [cloud.v1.api.QuotaReservationView](#cloud-v1-api-quotareservationview)
  - [cloud.v1.api.QuotaView](#cloud-v1-api-quotaview)
  - [cloud.v1.api.RatingEntry](#cloud-v1-api-ratingentry)
  - [cloud.v1.api.RatingFilter](#cloud-v1-api-ratingfilter)
  - [cloud.v1.api.RefreshQuotasRequest](#cloud-v1-api-refreshquotasrequest)
  - [cloud.v1.api.RefreshQuotasResponse](#cloud-v1-api-refreshquotasresponse)
  - [cloud.v1.api.RefreshRequest](#cloud-v1-api-refreshrequest)
  - [cloud.v1.api.RefreshResponse](#cloud-v1-api-refreshresponse)
  - [cloud.v1.api.RegisterRequest](#cloud-v1-api-registerrequest)
  - [cloud.v1.api.RegisterResponse](#cloud-v1-api-registerresponse)
  - [cloud.v1.api.RegistrationRequest](#cloud-v1-api-registrationrequest)
  - [cloud.v1.api.RegistrationRequestStatus](#cloud-v1-api-registrationrequeststatus)
  - [cloud.v1.api.RemoveFavoriteRequest](#cloud-v1-api-removefavoriterequest)
  - [cloud.v1.api.RemoveFavoriteResponse](#cloud-v1-api-removefavoriteresponse)
  - [cloud.v1.api.RequestPasswordResetRequest](#cloud-v1-api-requestpasswordresetrequest)
  - [cloud.v1.api.RequestPasswordResetResponse](#cloud-v1-api-requestpasswordresetresponse)
  - [cloud.v1.api.ResendVerificationRequest](#cloud-v1-api-resendverificationrequest)
  - [cloud.v1.api.ResendVerificationResponse](#cloud-v1-api-resendverificationresponse)
  - [cloud.v1.api.ResetPasswordRequest](#cloud-v1-api-resetpasswordrequest)
  - [cloud.v1.api.ResetPasswordResponse](#cloud-v1-api-resetpasswordresponse)
  - [cloud.v1.api.ResolveLogRefRequest](#cloud-v1-api-resolvelogrefrequest)
  - [cloud.v1.api.ResolveLogRefResponse](#cloud-v1-api-resolvelogrefresponse)
  - [cloud.v1.api.RevokeApiTokenRequest](#cloud-v1-api-revokeapitokenrequest)
  - [cloud.v1.api.RevokeApiTokenResponse](#cloud-v1-api-revokeapitokenresponse)
  - [cloud.v1.api.RevokeShareRequest](#cloud-v1-api-revokesharerequest)
  - [cloud.v1.api.RevokeShareResponse](#cloud-v1-api-revokeshareresponse)
  - [cloud.v1.api.RunColumn](#cloud-v1-api-runcolumn)
  - [cloud.v1.api.SetShareExpiryRequest](#cloud-v1-api-setshareexpiryrequest)
  - [cloud.v1.api.SetShareExpiryResponse](#cloud-v1-api-setshareexpiryresponse)
  - [cloud.v1.api.SetTenantProviderSettingsRequest](#cloud-v1-api-settenantprovidersettingsrequest)
  - [cloud.v1.api.ShellClientFrame](#cloud-v1-api-shellclientframe)
  - [cloud.v1.api.ShellServerFrame](#cloud-v1-api-shellserverframe)
  - [cloud.v1.api.ShellStart](#cloud-v1-api-shellstart)
  - [cloud.v1.api.SsoButton](#cloud-v1-api-ssobutton)
  - [cloud.v1.api.StartRunRequest](#cloud-v1-api-startrunrequest)
  - [cloud.v1.api.StartRunResponse](#cloud-v1-api-startrunresponse)
  - [cloud.v1.api.StartSSORequest](#cloud-v1-api-startssorequest)
  - [cloud.v1.api.StartSSOResponse](#cloud-v1-api-startssoresponse)
  - [cloud.v1.api.StatusCounts](#cloud-v1-api-statuscounts)
  - [cloud.v1.api.StreamLogsRequest](#cloud-v1-api-streamlogsrequest)
  - [cloud.v1.api.StreamTestRunOverviewRequest](#cloud-v1-api-streamtestrunoverviewrequest)
  - [cloud.v1.api.SubmitRegistrationRequestRequest](#cloud-v1-api-submitregistrationrequestrequest)
  - [cloud.v1.api.SubmitRegistrationRequestResponse](#cloud-v1-api-submitregistrationrequestresponse)
  - [cloud.v1.api.TenantDashboard](#cloud-v1-api-tenantdashboard)
  - [cloud.v1.api.TestRunOverviewSnapshot](#cloud-v1-api-testrunoverviewsnapshot)
  - [cloud.v1.api.TokenPair](#cloud-v1-api-tokenpair)
  - [cloud.v1.api.TransferTenantOwnershipRequest](#cloud-v1-api-transfertenantownershiprequest)
  - [cloud.v1.api.TransferTenantOwnershipResponse](#cloud-v1-api-transfertenantownershipresponse)
  - [cloud.v1.api.UnlinkExternalIdentityRequest](#cloud-v1-api-unlinkexternalidentityrequest)
  - [cloud.v1.api.UnlinkExternalIdentityResponse](#cloud-v1-api-unlinkexternalidentityresponse)
  - [cloud.v1.api.UpcomingSuite](#cloud-v1-api-upcomingsuite)
  - [cloud.v1.api.UpdateAccountRequest](#cloud-v1-api-updateaccountrequest)
  - [cloud.v1.api.UpdateAccountResponse](#cloud-v1-api-updateaccountresponse)
  - [cloud.v1.api.UpdateIdentityProviderRequest](#cloud-v1-api-updateidentityproviderrequest)
  - [cloud.v1.api.UpdateIdentityProviderResponse](#cloud-v1-api-updateidentityproviderresponse)
  - [cloud.v1.api.UpdateMembershipRequest](#cloud-v1-api-updatemembershiprequest)
  - [cloud.v1.api.UpdateMembershipResponse](#cloud-v1-api-updatemembershipresponse)
  - [cloud.v1.api.UpdateRoleRequest](#cloud-v1-api-updaterolerequest)
  - [cloud.v1.api.UpdateRoleResponse](#cloud-v1-api-updateroleresponse)
  - [cloud.v1.api.UpdateSystemSettingsRequest](#cloud-v1-api-updatesystemsettingsrequest)
  - [cloud.v1.api.UpdateSystemSettingsResponse](#cloud-v1-api-updatesystemsettingsresponse)
  - [cloud.v1.api.UpdateTenantRequest](#cloud-v1-api-updatetenantrequest)
  - [cloud.v1.api.UpdateTenantResponse](#cloud-v1-api-updatetenantresponse)
  - [cloud.v1.api.UpdateTenantSettingsRequest](#cloud-v1-api-updatetenantsettingsrequest)
  - [cloud.v1.api.UpdateTenantSettingsResponse](#cloud-v1-api-updatetenantsettingsresponse)
  - [cloud.v1.api.VerifyEmailRequest](#cloud-v1-api-verifyemailrequest)
  - [cloud.v1.api.VerifyEmailResponse](#cloud-v1-api-verifyemailresponse)

<a name="cloud-v1-api-services"></a>
## Services

<a name="cloud-v1-api-messages"></a>
## Messages

<a name="cloud-v1-api-addfavoriterequest"></a>
### cloud.v1.api.AddFavoriteRequest

<pre>
//AddFavorite marks (kind, target_id) as a favorite of the caller. Idempotent:
//favoriting an already-favorited row is a no-op (returns the existing record).
//The server validates the target exists and is in the caller's tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>kind</td>
<td><a href="../common/README.md#cloud-v1-common-favoritekind">cloud.v1.common.FavoriteKind</a></td>
<td><pre>
//kind is the favoritable resource type (must be a defined, non-zero kind).<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>target_id</td>
<td>string</td>
<td><pre>
//target_id is the id of the row being favorited, within `kind`.<br>

json_name: targetId
go_name: TargetId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the favorite; favorites are tenant-local and personal.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-addfavoriteresponse"></a>
### cloud.v1.api.AddFavoriteResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>favorite</td>
<td><a href="../models/README.md#cloud-v1-models-favoriterecord">cloud.v1.models.FavoriteRecord</a></td>
<td><pre>
//favorite is the resulting (or pre-existing) favorite join record.<br>

json_name: favorite
go_name: Favorite</pre></td>
</tr>
</table>



<a name="cloud-v1-api-cancelrunrequest"></a>
### cloud.v1.api.CancelRunRequest

<pre>
//CancelRunRequest requests cancellation of an in-flight recipe run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the run record identifier to cancel.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-cancelrunresponse"></a>
### cloud.v1.api.CancelRunResponse

<pre>
//CancelRunResponse is empty; cancellation is asynchronous — poll the run
//record (or ListRuns/Overview) to observe the resulting status.
</pre>



<a name="cloud-v1-api-catalogentry"></a>
### cloud.v1.api.CatalogEntry

<pre>
//CatalogEntry is one grantable Permission plus a human label for the UI.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>label</td>
<td>string</td>
<td><pre>
//label is a display string for the role editor, e.g. "Create role".<br>

json_name: label
go_name: Label</pre></td>
</tr><tr>
<td>permission</td>
<td><a href="../iam/README.md#cloud-v1-iam-permission">cloud.v1.iam.Permission</a></td>
<td><pre>
//permission is one grantable {resource, action} pair.<br>

json_name: permission
go_name: Permission</pre></td>
</tr>
</table>



<a name="cloud-v1-api-changepasswordrequest"></a>
### cloud.v1.api.ChangePasswordRequest

<pre>
//ChangePasswordRequest is the authenticated self-service rotation: the caller
//proves knowledge of old_password and sets new_password. Always operates on
//the caller's own account (from the token). Not idempotent — a replay fails
//once old_password no longer matches.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>new_password</td>
<td>string</td>
<td><pre>
//new_password is the replacement password to set.<br>

json_name: newPassword
go_name: NewPassword</pre></td>
</tr><tr>
<td>old_password</td>
<td>string</td>
<td><pre>
//old_password is the caller's current password, verified before the change.<br>

json_name: oldPassword
go_name: OldPassword</pre></td>
</tr>
</table>



<a name="cloud-v1-api-changepasswordresponse"></a>
### cloud.v1.api.ChangePasswordResponse

<pre>
//ChangePasswordResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-checkreciperequest"></a>
### cloud.v1.api.CheckRecipeRequest

<pre>
//CheckRecipeRequest compiles an already-stored recipe bundle in
//check-mode. Diagnostics from user input are NEVER surfaced as an RPC
//error — only as entries in CheckRecipeResponse.diagnostics.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the recipe record identifier whose stored bundle is checked.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-checkreciperesponse"></a>
### cloud.v1.api.CheckRecipeResponse

<pre>
//CheckRecipeResponse returns the check-mode compile diagnostics for the
//stored bundle. An empty list means the bundle compiles cleanly.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>diagnostics</td>
<td><a href="../dsl/README.md#cloud-v1-dsl-diagnostic">cloud.v1.dsl.Diagnostic</a></td>
<td><pre>
//diagnostics is the result of check-mode compilation (see
//cloud.v1.dsl.DslService.Check for the same diagnostic model).<br>

json_name: diagnostics
go_name: Diagnostics</pre></td>
</tr>
</table>



<a name="cloud-v1-api-comparerunsrequest"></a>
### cloud.v1.api.CompareRunsRequest

<pre>
//CompareRunsRequest asks for a side-by-side comparison of two or more runs.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_ids</td>
<td>string</td>
<td><pre>
//run_ids are the runs to compare in display order; run_ids[0] is the
//baseline.<br>

json_name: runIds
go_name: RunIds</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request; all runs must belong to this tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-comparerunsresponse"></a>
### cloud.v1.api.CompareRunsResponse

<pre>
//CompareRunsResponse wraps the assembled comparison view.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>view</td>
<td><a href="#cloud-v1-api-compareview">cloud.v1.api.CompareView</a></td>
<td><pre>
//view is the side-by-side comparison (columns + metric diff).<br>

json_name: view
go_name: View</pre></td>
</tr>
</table>



<a name="cloud-v1-api-compareview"></a>
### cloud.v1.api.CompareView

<pre>
//CompareView is the full side-by-side comparison payload: one config column
//per run plus the per-metric diff across all of them.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>columns</td>
<td><a href="#cloud-v1-api-runcolumn">cloud.v1.api.RunColumn</a></td>
<td><pre>
//columns are the per-run config columns, aligned 1:1 with the request
//run_ids; columns[0] is the baseline.<br>

json_name: columns
go_name: Columns</pre></td>
</tr><tr>
<td>metrics</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-comparison">cloud.v1.monitor.Comparison</a></td>
<td><pre>
//metrics is the per-metric diff across the runs (baseline = run_ids[0]).<br>

json_name: metrics
go_name: Metrics</pre></td>
</tr>
</table>



<a name="cloud-v1-api-completessorequest"></a>
### cloud.v1.api.CompleteSSORequest

<pre>
//CompleteSSORequest is the OIDC callback target. The IdP redirects the user
//back with code + state as query params; the http layer maps them here. The
//server validates state, exchanges code for the IdP tokens, reads the subject,
//resolves (or JIT-provisions, per the provider) the Account, and mints our own
//TokenPair.

//The OIDC nonce is NOT carried here: the server generates it alongside state +
//PKCE in StartSSO, stashes it server-side, and validates the id_token's nonce
//claim during the code exchange — clients never see it.

//HTTP NOTE. This RPC returns the TokenPair as JSON. Since the IdP redirects a
//BROWSER to the callback, the http layer wrapping this RPC is responsible for
//turning that response into a browser-friendly outcome (e.g. a 302 to the SPA
//with the tokens, or a Set-Cookie) rather than rendering raw JSON. That same
//layer also maps the callback route's <slug> (/auth/sso/<slug>/callback) onto
//the provider_id field below — the RPC keys on id, the URL on slug.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>code</td>
<td>string</td>
<td><pre>
//code is the IdP authorization code to exchange for tokens.<br>

json_name: code
go_name: Code</pre></td>
</tr><tr>
<td>provider_id</td>
<td>string</td>
<td><pre>
//provider_id is the OIDC provider this callback belongs to (URL maps slug
//-> id).<br>

json_name: providerId
go_name: ProviderId</pre></td>
</tr><tr>
<td>state</td>
<td>string</td>
<td><pre>
//state is the CSRF token from StartSSO, validated against the stashed value.<br>

json_name: state
go_name: State</pre></td>
</tr>
</table>



<a name="cloud-v1-api-completessoresponse"></a>
### cloud.v1.api.CompleteSSOResponse

<pre>
//CompleteSSOResponse returns our own TokenPair for the resolved account.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tokens</td>
<td><a href="#cloud-v1-api-tokenpair">cloud.v1.api.TokenPair</a></td>
<td><pre>
//tokens is the minted access + refresh credential pair.<br>

json_name: tokens
go_name: Tokens</pre></td>
</tr>
</table>



<a name="cloud-v1-api-completeuploadrequest"></a>
### cloud.v1.api.CompleteUploadRequest

<pre>
//CompleteUpload finalizes after the client PUT the blob: the server verifies
//size + sha256 and flips the record to READY (or FAILED). Idempotent.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the package record being finalized.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the package's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-completeuploadresponse"></a>
### cloud.v1.api.CompleteUploadResponse

<pre>
//CompleteUploadResponse returns the record after verification (READY/FAILED).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>package</td>
<td><a href="../models/README.md#cloud-v1-models-packagerecord">cloud.v1.models.PackageRecord</a></td>
<td><pre>
//package is the finalized record (now STATUS_READY or STATUS_FAILED).<br>

json_name: package
go_name: Package</pre></td>
</tr>
</table>



<a name="cloud-v1-api-confirmpasswordresetrequest"></a>
### cloud.v1.api.ConfirmPasswordResetRequest

<pre>
//ConfirmPasswordResetRequest completes the forgot-password flow: it consumes
//the emailed token and sets new_password. PUBLIC; the token is the credential.
//Not idempotent — the token is single-use.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>new_password</td>
<td>string</td>
<td><pre>
//new_password is the replacement password to set.<br>

json_name: newPassword
go_name: NewPassword</pre></td>
</tr><tr>
<td>token</td>
<td>string</td>
<td><pre>
//token is the single-use emailed reset token (the credential).<br>

json_name: token
go_name: Token</pre></td>
</tr>
</table>



<a name="cloud-v1-api-confirmpasswordresetresponse"></a>
### cloud.v1.api.ConfirmPasswordResetResponse

<pre>
//ConfirmPasswordResetResponse is empty; success is signalled by the absence of
//error.
</pre>



<a name="cloud-v1-api-createaccountrequest"></a>
### cloud.v1.api.CreateAccountRequest

<pre>
//CreateAccountRequest provisions a new global identity (the admin-only path;
//the public path is RegisterRequest). The password is the only secret accepted
//here; it is hashed and stored by the auth subsystem and never echoed back on
//the returned Account.

//An account needs at least ONE way to authenticate: the service rejects a
//request with neither password nor link, since that yields an account no one
//can ever log into. Supply a password, a link, or both.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>email</td>
<td>string</td>
<td><pre>
//email is the new account's contact + login address (format-validated).<br>

json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>is_admin</td>
<td>bool</td>
<td><pre>
//is_admin may only be set by an existing platform admin; ignored otherwise.<br>

json_name: isAdmin
go_name: IsAdmin</pre></td>
</tr><tr>
<td>link</td>
<td><a href="#cloud-v1-api-externalidentitylink">cloud.v1.api.ExternalIdentityLink</a></td>
<td><pre>
//link, when set, pre-links the new account to an external identity in the
//same call — the admin asserts the (provider, subject) binding so the user
//can log in via SSO immediately, without JIT provisioning.<br>

json_name: link
go_name: Link</pre></td>
</tr><tr>
<td>nickname</td>
<td>string</td>
<td><pre>
//nickname is the URL/handle-safe display handle (also an alternate login).<br>

json_name: nickname
go_name: Nickname</pre></td>
</tr><tr>
<td>password</td>
<td>string</td>
<td><pre>
//password is optional: omit it to create an SSO-only account that signs in
//exclusively through a linked external identity (see link).<br>

json_name: password
go_name: Password</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createaccountresponse"></a>
### cloud.v1.api.CreateAccountResponse

<pre>
//CreateAccountResponse returns the newly provisioned account (no secret).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account</td>
<td><a href="../iam/README.md#cloud-v1-iam-account">cloud.v1.iam.Account</a></td>
<td><pre>
//account is the created global identity (credentials never echoed).<br>

json_name: account
go_name: Account</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createapitokenrequest"></a>
### cloud.v1.api.CreateApiTokenRequest

<pre>
//CreateApiTokenRequest mints a new token for account_id. The caller must be
//that account or a platform admin (enforced server-side), so a user creates
//their own and an admin can provision service tokens for others.

//For a SERVICE token, permissions is the requested subset; the service rejects
//any permission the owner does not currently hold (a token can never exceed its
//owner). For a PERSONAL token, permissions MUST be empty — it inherits the
//account's authority.
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
//account_id is the account the token authenticates as.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is a human-readable label for the token.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>permissions</td>
<td><a href="../iam/README.md#cloud-v1-iam-permission">cloud.v1.iam.Permission</a></td>
<td><pre>
//permissions is the requested grant for a SERVICE token; leave empty for a
//PERSONAL token.<br>

json_name: permissions
go_name: Permissions</pre></td>
</tr><tr>
<td>ttl</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//ttl is the optional lifetime from creation. Omit (or zero) for a token
//that never expires.<br>

json_name: ttl
go_name: Ttl</pre></td>
</tr><tr>
<td>type</td>
<td><a href="../iam/README.md#cloud-v1-iam-apitokentype">cloud.v1.iam.ApiTokenType</a></td>
<td><pre>
//type is PERSONAL (inherits the account's authority) or SERVICE (a capped
//subset); must be defined, non-zero.<br>

json_name: type
go_name: Type</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createapitokenresponse"></a>
### cloud.v1.api.CreateApiTokenResponse

<pre>
//CreateApiTokenResponse carries the created token's metadata AND the plaintext
//secret. The secret is shown only here and never again — the caller must store
//it now.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>secret</td>
<td>string</td>
<td><pre>
//secret is the full plaintext token (prefix + secret) to send as a bearer
//credential. Returned once; the server keeps only its hash.<br>

json_name: secret
go_name: Secret</pre></td>
</tr><tr>
<td>token</td>
<td><a href="../iam/README.md#cloud-v1-iam-apitoken">cloud.v1.iam.ApiToken</a></td>
<td><pre>
//token is the created token's metadata (no secret).<br>

json_name: token
go_name: Token</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createidentityproviderrequest"></a>
### cloud.v1.api.CreateIdentityProviderRequest

<pre>
//CreateIdentityProviderRequest configures a new OIDC provider. client_secret
//is write-only: accepted here, stored server-side, never returned on the
//IdentityProvider. The provider is created ENABLED (IdentityProvider.disabled
//defaults false); hide it later via UpdateIdentityProvider if needed.
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
//allowed_domains restricts which email domains may sign in via this IdP.<br>

json_name: allowedDomains
go_name: AllowedDomains</pre></td>
</tr><tr>
<td>auto_provision</td>
<td>bool</td>
<td><pre>
//auto_provision enables JIT account creation for first-time SSO users.<br>

json_name: autoProvision
go_name: AutoProvision</pre></td>
</tr><tr>
<td>client_id</td>
<td>string</td>
<td><pre>
//client_id is the OAuth client identifier registered at the IdP.<br>

json_name: clientId
go_name: ClientId</pre></td>
</tr><tr>
<td>client_secret</td>
<td>string</td>
<td><pre>
//client_secret is the OAuth client secret; write-only (never returned).<br>

json_name: clientSecret
go_name: ClientSecret</pre></td>
</tr><tr>
<td>display_name</td>
<td>string</td>
<td><pre>
//display_name is the login-button label shown to users.<br>

json_name: displayName
go_name: DisplayName</pre></td>
</tr><tr>
<td>issuer</td>
<td>string</td>
<td><pre>
//issuer is the OIDC issuer URL (https, used for discovery).<br>

json_name: issuer
go_name: Issuer</pre></td>
</tr><tr>
<td>scopes</td>
<td>string</td>
<td><pre>
//scopes are the OIDC scopes to request at authorize time.<br>

json_name: scopes
go_name: Scopes</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
//slug is the url-safe key used in the SSO callback route.<br>

json_name: slug
go_name: Slug</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createidentityproviderresponse"></a>
### cloud.v1.api.CreateIdentityProviderResponse

<pre>
//CreateIdentityProviderResponse returns the created provider (no secret).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="../iam/README.md#cloud-v1-iam-identityprovider">cloud.v1.iam.IdentityProvider</a></td>
<td><pre>
//provider is the newly created (enabled) OIDC provider config.<br>

json_name: provider
go_name: Provider</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createmembershiprequest"></a>
### cloud.v1.api.CreateMembershipRequest

<pre>
//CreateMembershipRequest adds an account to a tenant with a set of roles — the
//"invite/add member" operation. Referenced roles must be SCOPE_TENANT roles of
//the same tenant_id or SCOPE_PLATFORM roles.
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
//account_id is the account being added to the tenant.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr><tr>
<td>role_ids</td>
<td>string</td>
<td><pre>
//role_ids are the granted roles (SCOPE_TENANT of this tenant, or
//SCOPE_PLATFORM); at least one is required.<br>

json_name: roleIds
go_name: RoleIds</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id is the tenant the account joins.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createmembershipresponse"></a>
### cloud.v1.api.CreateMembershipResponse

<pre>
//CreateMembershipResponse returns the created membership.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>membership</td>
<td><a href="../iam/README.md#cloud-v1-iam-membership">cloud.v1.iam.Membership</a></td>
<td><pre>
//membership is the newly created account-in-tenant grant.<br>

json_name: membership
go_name: Membership</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createpackageuploadrequest"></a>
### cloud.v1.api.CreatePackageUploadRequest

<pre>
//CreatePackageUploadRequest declares the package metadata and mints a presigned
//upload; the blob is PUT directly to object storage afterward.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>arch</td>
<td>string</td>
<td><pre>
//arch is the target CPU architecture the package is built for.<br>

json_name: arch
go_name: Arch</pre></td>
</tr><tr>
<td>format</td>
<td><a href="../models/README.md#cloud-v1-models-packagerecord-format">cloud.v1.models.PackageRecord.Format</a></td>
<td><pre>
//format is the package format (.deb / binary); must be defined, non-zero.<br>

json_name: format
go_name: Format</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the package's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>os</td>
<td>string</td>
<td><pre>
//os is the target operating system the package is built for.<br>

json_name: os
go_name: Os</pre></td>
</tr><tr>
<td>sha256</td>
<td>string</td>
<td><pre>
//sha256 is the declared blob hash; verified on CompleteUpload against the
//uploaded object.<br>

json_name: sha256
go_name: Sha256</pre></td>
</tr><tr>
<td>size_bytes</td>
<td>uint64</td>
<td><pre>
//size_bytes is the declared blob size; verified on CompleteUpload against
//the uploaded object.<br>

json_name: sizeBytes
go_name: SizeBytes</pre></td>
</tr><tr>
<td>target_db_kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//target_db_kind is the database engine this package builds/installs.<br>

json_name: targetDbKind
go_name: TargetDbKind</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the package; packages are tenant-private.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>version</td>
<td>string</td>
<td><pre>
//version is the package version string.<br>

json_name: version
go_name: Version</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createpackageuploadresponse"></a>
### cloud.v1.api.CreatePackageUploadResponse

<pre>
//CreatePackageUploadResponse returns the pending record plus the presigned PUT
//url the client uploads the blob to.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>package</td>
<td><a href="../models/README.md#cloud-v1-models-packagerecord">cloud.v1.models.PackageRecord</a></td>
<td><pre>
//package is the created record (STATUS_UPLOADING).<br>

json_name: package
go_name: Package</pre></td>
</tr><tr>
<td>upload_url</td>
<td>string</td>
<td><pre>
//upload_url is the presigned PUT url the client uploads the blob to.<br>

json_name: uploadUrl
go_name: UploadUrl</pre></td>
</tr><tr>
<td>upload_url_expires_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//upload_url_expires_at is when the upload url stops working.<br>

json_name: uploadUrlExpiresAt
go_name: UploadUrlExpiresAt</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createreciperequest"></a>
### cloud.v1.api.CreateRecipeRequest

<pre>
//CreateRecipeRequest persists a new recipe bundle under a tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>recipe</td>
<td><a href="../models/README.md#cloud-v1-models-reciperecord">cloud.v1.models.RecipeRecord</a></td>
<td><pre>
//recipe is the bundle to persist. Server assigns entity.id / tenant_id
/// timings.<br>

json_name: recipe
go_name: Recipe</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createreciperesponse"></a>
### cloud.v1.api.CreateRecipeResponse

<pre>
//CreateRecipeResponse returns the newly created recipe record.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>recipe</td>
<td><a href="../models/README.md#cloud-v1-models-reciperecord">cloud.v1.models.RecipeRecord</a></td>
<td><pre>
//recipe is the persisted record with server-assigned fields populated.<br>

json_name: recipe
go_name: Recipe</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createrolerequest"></a>
### cloud.v1.api.CreateRoleRequest

<pre>
//CreateRoleRequest defines a custom role. tenant_id MUST be set for
//SCOPE_TENANT and empty for SCOPE_PLATFORM (enforced server-side). System
//roles are seeded by the server and cannot be created here.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the role's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>permissions</td>
<td><a href="../iam/README.md#cloud-v1-iam-permission">cloud.v1.iam.Permission</a></td>
<td><pre>
//permissions is the role's granted permission set.<br>

json_name: permissions
go_name: Permissions</pre></td>
</tr><tr>
<td>scope</td>
<td><a href="../iam/README.md#cloud-v1-iam-scope">cloud.v1.iam.Scope</a></td>
<td><pre>
//scope is SCOPE_TENANT or SCOPE_PLATFORM (must be defined, non-zero).<br>

json_name: scope
go_name: Scope</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id is required for SCOPE_TENANT, empty for SCOPE_PLATFORM.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createroleresponse"></a>
### cloud.v1.api.CreateRoleResponse

<pre>
//CreateRoleResponse returns the created role.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>role</td>
<td><a href="../iam/README.md#cloud-v1-iam-role">cloud.v1.iam.Role</a></td>
<td><pre>
//role is the newly created custom role.<br>

json_name: role
go_name: Role</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createsharerequest"></a>
### cloud.v1.api.CreateShareRequest

<pre>
//CreateShareRequest mints a new share (token + first snapshot) for a run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>target</td>
<td><a href="../models/README.md#cloud-v1-models-sharerecord-target">cloud.v1.models.ShareRecord.Target</a></td>
<td><pre>
//target is what to share (test run / suite run + id).<br>

json_name: target
go_name: Target</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the share; shares are tenant-scoped.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>ttl</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//ttl is the share lifetime. 0 / unset -> server default (1 week). Explicit 0
//to mean "never" is allowed but the client SHOULD warn the user: it is
//insecure and keeps the background refresh running forever.<br>

json_name: ttl
go_name: Ttl</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createshareresponse"></a>
### cloud.v1.api.CreateShareResponse

<pre>
//CreateShareResponse returns the created share record (with its token).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>share</td>
<td><a href="../models/README.md#cloud-v1-models-sharerecord">cloud.v1.models.ShareRecord</a></td>
<td><pre>
//share is the newly created share record.<br>

json_name: share
go_name: Share</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createtenantrequest"></a>
### cloud.v1.api.CreateTenantRequest

<pre>
//CreateTenantRequest creates a workspace. The caller becomes owner and the
//server seeds an owner Membership so the creator can immediately enter the
//tenant. slug must be unique and url-safe (see iam/tenant.proto).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the tenant's human-readable display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
//slug is the unique, url-safe routing key (/t/<slug>).<br>

json_name: slug
go_name: Slug</pre></td>
</tr>
</table>



<a name="cloud-v1-api-createtenantresponse"></a>
### cloud.v1.api.CreateTenantResponse

<pre>
//CreateTenantResponse returns the created tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant</td>
<td><a href="../iam/README.md#cloud-v1-iam-tenant">cloud.v1.iam.Tenant</a></td>
<td><pre>
//tenant is the newly created workspace (caller seeded as owner).<br>

json_name: tenant
go_name: Tenant</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deleteaccountrequest"></a>
### cloud.v1.api.DeleteAccountRequest

<pre>
//DeleteAccountRequest removes an account. The account may be referenced by
//Memberships, ExternalIdentities, and owned Tenants (Tenant.owner_account_id);
//the service layer decides the cascade: linked Memberships and
//ExternalIdentities are removed with it, but a delete is REJECTED while the
//account still owns any Tenant — transfer ownership first
//(TransferTenantOwnership) so no tenant is orphaned.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the account to remove.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deleteaccountresponse"></a>
### cloud.v1.api.DeleteAccountResponse

<pre>
//DeleteAccountResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-deleteidentityproviderrequest"></a>
### cloud.v1.api.DeleteIdentityProviderRequest

<pre>
//DeleteIdentityProviderRequest removes a provider by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the provider to remove.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deleteidentityproviderresponse"></a>
### cloud.v1.api.DeleteIdentityProviderResponse

<pre>
//DeleteIdentityProviderResponse is empty; success is signalled by the absence
//of error.
</pre>



<a name="cloud-v1-api-deletemembershiprequest"></a>
### cloud.v1.api.DeleteMembershipRequest

<pre>
//DeleteMembershipRequest removes a member from a tenant (revokes access).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the membership to remove (revokes the account's tenant access).<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deletemembershipresponse"></a>
### cloud.v1.api.DeleteMembershipResponse

<pre>
//DeleteMembershipResponse is empty; success is signalled by the absence of
//error.
</pre>



<a name="cloud-v1-api-deletepackagerequest"></a>
### cloud.v1.api.DeletePackageRequest

<pre>
//DeletePackageRequest removes one package by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the package to remove.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the package's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deletepackageresponse"></a>
### cloud.v1.api.DeletePackageResponse

<pre>
//DeletePackageResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-deletereciperequest"></a>
### cloud.v1.api.DeleteRecipeRequest

<pre>
//DeleteRecipeRequest soft-deletes a recipe record by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the recipe record identifier to delete.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deletereciperesponse"></a>
### cloud.v1.api.DeleteRecipeResponse

<pre>
//DeleteRecipeResponse is empty; soft-delete success is signalled by a
//non-error reply.
</pre>



<a name="cloud-v1-api-deleterolerequest"></a>
### cloud.v1.api.DeleteRoleRequest

<pre>
//DeleteRoleRequest removes a role by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the role to remove.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deleteroleresponse"></a>
### cloud.v1.api.DeleteRoleResponse

<pre>
//DeleteRoleResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-deleterunrequest"></a>
### cloud.v1.api.DeleteRunRequest

<pre>
//DeleteRunRequest deletes a run record by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the run record identifier to delete.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deleterunresponse"></a>
### cloud.v1.api.DeleteRunResponse

<pre>
//DeleteRunResponse is empty; success is signalled by a non-error reply.
</pre>



<a name="cloud-v1-api-deletesharerequest"></a>
### cloud.v1.api.DeleteShareRequest

<pre>
//DeleteShareRequest removes one share by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the share to remove.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the share's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deleteshareresponse"></a>
### cloud.v1.api.DeleteShareResponse

<pre>
//DeleteShareResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-deletetenantrequest"></a>
### cloud.v1.api.DeleteTenantRequest

<pre>
//DeleteTenantRequest removes a tenant by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the tenant to remove.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-deletetenantresponse"></a>
### cloud.v1.api.DeleteTenantResponse

<pre>
//DeleteTenantResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-externalidentitylink"></a>
### cloud.v1.api.ExternalIdentityLink

<pre>
//ExternalIdentityLink is an admin-asserted binding of a local account to an
//IdP subject, used by CreateAccount and LinkExternalIdentity. The server
//trusts the admin for the (provider_id, subject) pair; no SSO round-trip is
//performed to verify it.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>email</td>
<td>string</td>
<td><pre>
//email is the address to record on the link (display / domain checks).<br>

json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>provider_id</td>
<td>string</td>
<td><pre>
//provider_id is the IdentityProvider the subject belongs to.<br>

json_name: providerId
go_name: ProviderId</pre></td>
</tr><tr>
<td>subject</td>
<td>string</td>
<td><pre>
//subject is the IdP's stable subject identifier for the user.<br>

json_name: subject
go_name: Subject</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getaccountrequest"></a>
### cloud.v1.api.GetAccountRequest

<pre>
//GetAccountRequest fetches one account by id (RESOURCE_ACCOUNT/READ).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the account to fetch.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getaccountresponse"></a>
### cloud.v1.api.GetAccountResponse

<pre>
//GetAccountResponse returns the requested account.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account</td>
<td><a href="../iam/README.md#cloud-v1-iam-account">cloud.v1.iam.Account</a></td>
<td><pre>
//account is the fetched global identity.<br>

json_name: account
go_name: Account</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getidentityproviderrequest"></a>
### cloud.v1.api.GetIdentityProviderRequest

<pre>
//GetIdentityProviderRequest fetches one provider by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the provider to fetch.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getidentityproviderresponse"></a>
### cloud.v1.api.GetIdentityProviderResponse

<pre>
//GetIdentityProviderResponse returns the requested provider.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="../iam/README.md#cloud-v1-iam-identityprovider">cloud.v1.iam.IdentityProvider</a></td>
<td><pre>
//provider is the fetched OIDC provider config (no secret).<br>

json_name: provider
go_name: Provider</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getlogfacetsrequest"></a>
### cloud.v1.api.GetLogFacetsRequest

<pre>
//GetLogFacetsRequest asks for the distinct values (and their hit counts) of the
//log filter dimensions across the WHOLE run, not just a loaded buffer. The
//optional filter cross-narrows the facets (e.g. counts under the active
//machine/phase selection), matching the behaviour of the QueryLogs filter.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>filter</td>
<td><a href="#cloud-v1-api-logfilter">cloud.v1.api.LogFilter</a></td>
<td><pre>
//filter cross-narrows the facet values (AND-ed with the run id). Unset =
//facets over the whole run.<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id identifies the run whose log facets are requested.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getlogfacetsresponse"></a>
### cloud.v1.api.GetLogFacetsResponse

<pre>
//GetLogFacetsResponse returns the per-field distinct values across the run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>fields</td>
<td><a href="#cloud-v1-api-logfacetfield">cloud.v1.api.LogFacetField</a></td>
<td><pre>
json_name: fields
go_name: Fields</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getmembershiprequest"></a>
### cloud.v1.api.GetMembershipRequest

<pre>
//GetMembershipRequest fetches one membership by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the membership to fetch.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getmembershipresponse"></a>
### cloud.v1.api.GetMembershipResponse

<pre>
//GetMembershipResponse returns the requested membership.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>membership</td>
<td><a href="../iam/README.md#cloud-v1-iam-membership">cloud.v1.iam.Membership</a></td>
<td><pre>
//membership is the fetched account-in-tenant grant.<br>

json_name: membership
go_name: Membership</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getmyaccountrequest"></a>
### cloud.v1.api.GetMyAccountRequest

<pre>
//GetMyAccountRequest takes no arguments: it returns the CALLER's own Account,
//resolved from the token subject. This is the self-profile endpoint — any
//authenticated account may read itself without holding RESOURCE_ACCOUNT/READ.
</pre>



<a name="cloud-v1-api-getmyaccountresponse"></a>
### cloud.v1.api.GetMyAccountResponse

<pre>
//GetMyAccountResponse returns the caller's own account.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account</td>
<td><a href="../iam/README.md#cloud-v1-iam-account">cloud.v1.iam.Account</a></td>
<td><pre>
//account is the caller's own global identity.<br>

json_name: account
go_name: Account</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getmypermissionsrequest"></a>
### cloud.v1.api.GetMyPermissionsRequest

<pre>
//GetMyPermissionsRequest resolves the CALLER's effective Permissions in one
//tenant — the live union the gate computes per request, exposed so a UI can
//show/hide controls. The account comes from the token; tenant from the ref.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id is the tenant to resolve the caller's effective permissions in.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getmypermissionsresponse"></a>
### cloud.v1.api.GetMyPermissionsResponse

<pre>
//GetMyPermissionsResponse returns the caller's effective permissions.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>permissions</td>
<td><a href="../iam/README.md#cloud-v1-iam-permission">cloud.v1.iam.Permission</a></td>
<td><pre>
//permissions is the live union the gate computes for the caller in the
//tenant.<br>

json_name: permissions
go_name: Permissions</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getpackagerequest"></a>
### cloud.v1.api.GetPackageRequest

<pre>
//GetPackageRequest fetches one package by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the package to fetch.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the package's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getpackageresponse"></a>
### cloud.v1.api.GetPackageResponse

<pre>
//GetPackageResponse returns the requested package.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>package</td>
<td><a href="../models/README.md#cloud-v1-models-packagerecord">cloud.v1.models.PackageRecord</a></td>
<td><pre>
//package is the fetched package record.<br>

json_name: package
go_name: Package</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getpublicconfigrequest"></a>
### cloud.v1.api.GetPublicConfigRequest

<pre>
//GetPublicConfigRequest takes no arguments: it reads the public-safe subset of
//the singleton settings, with NO authentication. It exists so the sign-in /
//sign-up screens (which run before any token exists) can learn whether open
//self-registration and member tenant creation are enabled, and hide the
//affordances up front instead of only failing on submit.
</pre>



<a name="cloud-v1-api-getpublicconfigresponse"></a>
### cloud.v1.api.GetPublicConfigResponse

<pre>
//GetPublicConfigResponse returns only the non-sensitive flags safe to expose
//to an unauthenticated caller — never server_addr or any internal config.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>allow_member_tenant_creation</td>
<td>bool</td>
<td><pre>
//allow_member_tenant_creation mirrors
//PlatformSettings.allow_member_tenant_creation.<br>

json_name: allowMemberTenantCreation
go_name: AllowMemberTenantCreation</pre></td>
</tr><tr>
<td>allow_self_registration</td>
<td>bool</td>
<td><pre>
//allow_self_registration mirrors PlatformSettings.allow_self_registration:
//when false the public Register endpoint is closed.<br>

json_name: allowSelfRegistration
go_name: AllowSelfRegistration</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getpublicratingrequest"></a>
### cloud.v1.api.GetPublicRatingRequest

<pre>
//GetPublicRatingRequest selects and pages the public leaderboard.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>filter</td>
<td><a href="#cloud-v1-api-ratingfilter">cloud.v1.api.RatingFilter</a></td>
<td><pre>
//filter reuses the authenticated filter shape (metric_key + facets).<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>limit</td>
<td>uint32</td>
<td><pre>
//limit caps returned entries (<= 500); 0 -> server default.<br>

json_name: limit
go_name: Limit</pre></td>
</tr><tr>
<td>page_token</td>
<td>string</td>
<td><pre>
//page_token is the opaque cursor from a previous response.<br>

json_name: pageToken
go_name: PageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getpublicratingresponse"></a>
### cloud.v1.api.GetPublicRatingResponse

<pre>
//GetPublicRatingResponse returns one page of the public leaderboard.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entries</td>
<td><a href="#cloud-v1-api-publicratingentry">cloud.v1.api.PublicRatingEntry</a></td>
<td><pre>
//entries are the ranked benchmarks for this page (sensitive fields
//stripped).<br>

json_name: entries
go_name: Entries</pre></td>
</tr><tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getreciperequest"></a>
### cloud.v1.api.GetRecipeRequest

<pre>
//GetRecipeRequest fetches a single recipe record by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the recipe record identifier to fetch.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getreciperesponse"></a>
### cloud.v1.api.GetRecipeResponse

<pre>
//GetRecipeResponse returns the requested recipe record.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>recipe</td>
<td><a href="../models/README.md#cloud-v1-models-reciperecord">cloud.v1.models.RecipeRecord</a></td>
<td><pre>
//recipe is the requested record.<br>

json_name: recipe
go_name: Recipe</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getrolerequest"></a>
### cloud.v1.api.GetRoleRequest

<pre>
//GetRoleRequest fetches one role by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the role to fetch.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getroleresponse"></a>
### cloud.v1.api.GetRoleResponse

<pre>
//GetRoleResponse returns the requested role.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>role</td>
<td><a href="../iam/README.md#cloud-v1-iam-role">cloud.v1.iam.Role</a></td>
<td><pre>
//role is the fetched role.<br>

json_name: role
go_name: Role</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getrunmetricsrequest"></a>
### cloud.v1.api.GetRunMetricsRequest

<pre>
//GetRunMetricsRequest fetches the Metrics tab payload for a run. Multi-run
//comparison lives in compare.proto (CompareService.CompareRuns).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id identifies the run whose metrics are requested.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>window</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-timerange">cloud.v1.monitor.TimeRange</a></td>
<td><pre>
//window optionally scopes the aggregation to a sub-range of the run — e.g.
//a single workload segment's [started_at, finished_at] taken from the
//Overview pipeline — so averages exclude bootstrap/load time. Unset means
//the whole run window (the run record's start/finish).<br>

json_name: window
go_name: Window</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getrunmetricsresponse"></a>
### cloud.v1.api.GetRunMetricsResponse

<pre>
//GetRunMetricsResponse returns the run's metrics.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>metrics</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-runmetrics">cloud.v1.monitor.RunMetrics</a></td>
<td><pre>
//metrics is the run's aggregated metrics.<br>

json_name: metrics
go_name: Metrics</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getrunquotausagerequest"></a>
### cloud.v1.api.GetRunQuotaUsageRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getrunquotausageresponse"></a>
### cloud.v1.api.GetRunQuotaUsageResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>reservations</td>
<td><a href="#cloud-v1-api-quotareservationview">cloud.v1.api.QuotaReservationView</a></td>
<td><pre>
json_name: reservations
go_name: Reservations</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getsharerequest"></a>
### cloud.v1.api.GetShareRequest

<pre>
//GetShareRequest fetches one share by id.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the share to fetch.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the share's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getshareresponse"></a>
### cloud.v1.api.GetShareResponse

<pre>
//GetShareResponse returns the requested share.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>share</td>
<td><a href="../models/README.md#cloud-v1-models-sharerecord">cloud.v1.models.ShareRecord</a></td>
<td><pre>
//share is the fetched share record.<br>

json_name: share
go_name: Share</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getsharedrunrequest"></a>
### cloud.v1.api.GetSharedRunRequest

<pre>
//GetSharedRunRequest resolves one share by its public token.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>token</td>
<td>string</td>
<td><pre>
//token is the unguessable token from the share URL.<br>

json_name: token
go_name: Token</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getsharedrunresponse"></a>
### cloud.v1.api.GetSharedRunResponse

<pre>
//GetSharedRunResponse returns the limited public snapshot for the token.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>snapshot</td>
<td><a href="../models/README.md#cloud-v1-models-sharerecord-snapshot">cloud.v1.models.ShareRecord.Snapshot</a></td>
<td><pre>
//snapshot is the limited public snapshot (test or suite run) + when it was
//captured.<br>

json_name: snapshot
go_name: Snapshot</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getsystemratingrequest"></a>
### cloud.v1.api.GetSystemRatingRequest

<pre>
//GetSystemRatingRequest selects and pages the cross-system private board.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>filter</td>
<td><a href="#cloud-v1-api-ratingfilter">cloud.v1.api.RatingFilter</a></td>
<td><pre>
//filter selects and ranks the benchmark runs.<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>limit</td>
<td>uint32</td>
<td><pre>
//limit caps returned entries (<= 500); 0 -> server default.<br>

json_name: limit
go_name: Limit</pre></td>
</tr><tr>
<td>page_token</td>
<td>string</td>
<td><pre>
//page_token is the opaque cursor from a previous response.<br>

json_name: pageToken
go_name: PageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getsystemratingresponse"></a>
### cloud.v1.api.GetSystemRatingResponse

<pre>
//GetSystemRatingResponse returns one page of the system-wide board.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entries</td>
<td><a href="#cloud-v1-api-ratingentry">cloud.v1.api.RatingEntry</a></td>
<td><pre>
//entries are the ranked benchmarks for this page.<br>

json_name: entries
go_name: Entries</pre></td>
</tr><tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-getsystemsettingsrequest"></a>
### cloud.v1.api.GetSystemSettingsRequest

<pre>
//GetSystemSettingsRequest takes no arguments: it reads the singleton settings.
</pre>



<a name="cloud-v1-api-getsystemsettingsresponse"></a>
### cloud.v1.api.GetSystemSettingsResponse

<pre>
//GetSystemSettingsResponse returns the current control-plane settings.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="#cloud-v1-api-platformsettings">cloud.v1.api.PlatformSettings</a></td>
<td><pre>
//settings is the singleton platform configuration.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantdashboardrequest"></a>
### cloud.v1.api.GetTenantDashboardRequest

<pre>
//GetTenantDashboardRequest fetches the aggregated dashboard for a tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantdashboardresponse"></a>
### cloud.v1.api.GetTenantDashboardResponse

<pre>
//GetTenantDashboardResponse returns the computed dashboard payload.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>dashboard</td>
<td><a href="#cloud-v1-api-tenantdashboard">cloud.v1.api.TenantDashboard</a></td>
<td><pre>
//dashboard is the aggregated landing view.<br>

json_name: dashboard
go_name: Dashboard</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantratingrequest"></a>
### cloud.v1.api.GetTenantRatingRequest

<pre>
//GetTenantRatingRequest selects and pages one tenant's board.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>filter</td>
<td><a href="#cloud-v1-api-ratingfilter">cloud.v1.api.RatingFilter</a></td>
<td><pre>
//filter selects and ranks the benchmark runs.<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>limit</td>
<td>uint32</td>
<td><pre>
//limit caps returned entries (<= 500); 0 -> server default.<br>

json_name: limit
go_name: Limit</pre></td>
</tr><tr>
<td>page_token</td>
<td>string</td>
<td><pre>
//page_token is the opaque cursor from a previous response.<br>

json_name: pageToken
go_name: PageToken</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the board to one tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantratingresponse"></a>
### cloud.v1.api.GetTenantRatingResponse

<pre>
//GetTenantRatingResponse returns one page of the tenant's board.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entries</td>
<td><a href="#cloud-v1-api-ratingentry">cloud.v1.api.RatingEntry</a></td>
<td><pre>
//entries are the ranked benchmarks for this page.<br>

json_name: entries
go_name: Entries</pre></td>
</tr><tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantrequest"></a>
### cloud.v1.api.GetTenantRequest

<pre>
//GetTenantRequest fetches one tenant by EITHER id or slug (oneof). Slug lookup
//is the routing path: the gate resolves /t/<slug> to a Tenant via this.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the tenant's stable identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
//slug is the url-safe routing key (the /t/<slug> resolve path).<br>

json_name: slug
go_name: Slug</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantresponse"></a>
### cloud.v1.api.GetTenantResponse

<pre>
//GetTenantResponse returns the requested tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant</td>
<td><a href="../iam/README.md#cloud-v1-iam-tenant">cloud.v1.iam.Tenant</a></td>
<td><pre>
//tenant is the fetched workspace.<br>

json_name: tenant
go_name: Tenant</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantsettingsrequest"></a>
### cloud.v1.api.GetTenantSettingsRequest

<pre>
//GetTenantSettingsRequest fetches the settings for a tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettenantsettingsresponse"></a>
### cloud.v1.api.GetTenantSettingsResponse

<pre>
//GetTenantSettingsResponse returns the tenant's settings record.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="../models/README.md#cloud-v1-models-tenantsettingsrecord">cloud.v1.models.TenantSettingsRecord</a></td>
<td><pre>
//settings is the tenant's current settings.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettestrunoverviewrequest"></a>
### cloud.v1.api.GetTestRunOverviewRequest

<pre>
//GetTestRunOverviewRequest fetches the Overview tab payload for a run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id identifies the run whose overview is requested.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-gettestrunoverviewresponse"></a>
### cloud.v1.api.GetTestRunOverviewResponse

<pre>
//GetTestRunOverviewResponse returns the run's staged Overview snapshot.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>snapshot</td>
<td><a href="#cloud-v1-api-testrunoverviewsnapshot">cloud.v1.api.TestRunOverviewSnapshot</a></td>
<td><pre>
//snapshot is the full overview page state.<br>

json_name: snapshot
go_name: Snapshot</pre></td>
</tr>
</table>



<a name="cloud-v1-api-leavetenantrequest"></a>
### cloud.v1.api.LeaveTenantRequest

<pre>
//LeaveTenantRequest is the authenticated self-service exit: the CALLER drops
//their own Membership in the named tenant (the GitHub "leave org" action),
//distinct from an admin removing someone else (DeleteMembership). The owner
//MUST transfer ownership first (TransferTenantOwnership); the service rejects
//an owner trying to leave. Idempotent — leaving a tenant you are not in is a
//no-op.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id is the tenant the caller is leaving.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-leavetenantresponse"></a>
### cloud.v1.api.LeaveTenantResponse

<pre>
//LeaveTenantResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-linkexternalidentityrequest"></a>
### cloud.v1.api.LinkExternalIdentityRequest

<pre>
//LinkExternalIdentityRequest binds an existing account to an IdP subject
//(admin-asserted, like CreateAccount.link). Idempotent on (provider, subject).
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
//account_id is the existing account to bind to the IdP subject.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr><tr>
<td>link</td>
<td><a href="#cloud-v1-api-externalidentitylink">cloud.v1.api.ExternalIdentityLink</a></td>
<td><pre>
//link is the admin-asserted (provider, subject) binding to record.<br>

json_name: link
go_name: Link</pre></td>
</tr>
</table>



<a name="cloud-v1-api-linkexternalidentityresponse"></a>
### cloud.v1.api.LinkExternalIdentityResponse

<pre>
//LinkExternalIdentityResponse returns the created (or existing) link.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>identity</td>
<td><a href="../iam/README.md#cloud-v1-iam-externalidentity">cloud.v1.iam.ExternalIdentity</a></td>
<td><pre>
//identity is the resulting external-identity link.<br>

json_name: identity
go_name: Identity</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listaccountsrequest"></a>
### cloud.v1.api.ListAccountsRequest

<pre>
//ListAccountsRequest is platform-scoped (admin only).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>page_size</td>
<td>uint32</td>
<td><pre>
//page_size caps returned rows; 0 -> server default.<br>

json_name: pageSize
go_name: PageSize</pre></td>
</tr><tr>
<td>page_token</td>
<td>string</td>
<td><pre>
//page_token is the opaque cursor from a previous response.<br>

json_name: pageToken
go_name: PageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listaccountsresponse"></a>
### cloud.v1.api.ListAccountsResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>accounts</td>
<td><a href="../iam/README.md#cloud-v1-iam-account">cloud.v1.iam.Account</a></td>
<td><pre>
//accounts is this page of global identities.<br>

json_name: accounts
go_name: Accounts</pre></td>
</tr><tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listapitokensrequest"></a>
### cloud.v1.api.ListApiTokensRequest

<pre>
//ListApiTokensRequest lists the tokens of one account. The caller must be that
//account or a platform admin (enforced server-side) — the "my tokens" view as
//well as the admin one. Secrets are never included.
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
//account_id is the account whose tokens to list.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listapitokensresponse"></a>
### cloud.v1.api.ListApiTokensResponse

<pre>
//ListApiTokensResponse returns the account's token metadata (no secrets).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tokens</td>
<td><a href="../iam/README.md#cloud-v1-iam-apitoken">cloud.v1.iam.ApiToken</a></td>
<td><pre>
//tokens is the account's tokens (metadata only).<br>

json_name: tokens
go_name: Tokens</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listexternalidentitiesrequest"></a>
### cloud.v1.api.ListExternalIdentitiesRequest

<pre>
//ListExternalIdentitiesRequest lists the identities linked to one account. The
//caller must be that account or a platform admin (enforced server-side) — this
//is the "my linked logins" view as well as the admin one.
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
//account_id is the account whose linked identities to list.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listexternalidentitiesresponse"></a>
### cloud.v1.api.ListExternalIdentitiesResponse

<pre>
//ListExternalIdentitiesResponse returns the account's linked identities.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>identities</td>
<td><a href="../iam/README.md#cloud-v1-iam-externalidentity">cloud.v1.iam.ExternalIdentity</a></td>
<td><pre>
//identities are the external-identity links bound to the account.<br>

json_name: identities
go_name: Identities</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listfavoritesrequest"></a>
### cloud.v1.api.ListFavoritesRequest

<pre>
//ListFavorites returns the caller's favorites, optionally narrowed to one kind.
//This lists the raw join rows; to list the favorited ENTITIES themselves, use
//the target resource's List with EntityFilter.favorites_only = true instead.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>kind</td>
<td><a href="../common/README.md#cloud-v1-common-favoritekind">cloud.v1.common.FavoriteKind</a></td>
<td><pre>
//kind narrows to one kind; UNSPECIFIED = all kinds.<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>page</td>
<td><a href="../common/README.md#cloud-v1-common-page">cloud.v1.common.Page</a></td>
<td><pre>
//page is the pagination cursor/size.<br>

json_name: page
go_name: Page</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the listing to the caller's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listfavoritesresponse"></a>
### cloud.v1.api.ListFavoritesResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>favorites</td>
<td><a href="../models/README.md#cloud-v1-models-favoriterecord">cloud.v1.models.FavoriteRecord</a></td>
<td><pre>
//favorites are the caller's favorite join rows for this page.<br>

json_name: favorites
go_name: Favorites</pre></td>
</tr><tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listidentityprovidersrequest"></a>
### cloud.v1.api.ListIdentityProvidersRequest

<pre>
//ListIdentityProvidersRequest is PUBLIC (the login page needs the buttons
//before anyone is authenticated). It returns only ENABLED providers (those
//with IdentityProvider.disabled == false). The response is slim on purpose —
//only what a button needs — so admin-only config (domains, secret) never leaks
//here.
</pre>



<a name="cloud-v1-api-listidentityprovidersresponse"></a>
### cloud.v1.api.ListIdentityProvidersResponse

<pre>
//ListIdentityProvidersResponse returns the enabled providers' login buttons.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>buttons</td>
<td><a href="#cloud-v1-api-ssobutton">cloud.v1.api.SsoButton</a></td>
<td><pre>
//buttons are the enabled providers, slimmed to what a login button needs.<br>

json_name: buttons
go_name: Buttons</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listmembershipsrequest"></a>
### cloud.v1.api.ListMembershipsRequest

<pre>
//ListMembershipsRequest lists all members of one tenant.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>page_size</td>
<td>uint32</td>
<td><pre>
//page_size caps returned rows; 0 -> server default.<br>

json_name: pageSize
go_name: PageSize</pre></td>
</tr><tr>
<td>page_token</td>
<td>string</td>
<td><pre>
//page_token is the opaque cursor from a previous response.<br>

json_name: pageToken
go_name: PageToken</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id is the tenant whose members to list.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listmembershipsresponse"></a>
### cloud.v1.api.ListMembershipsResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>memberships</td>
<td><a href="../iam/README.md#cloud-v1-iam-membership">cloud.v1.iam.Membership</a></td>
<td><pre>
//memberships is this page of the tenant's members.<br>

json_name: memberships
go_name: Memberships</pre></td>
</tr><tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listmytenantsrequest"></a>
### cloud.v1.api.ListMyTenantsRequest

<pre>
//ListMyTenantsRequest returns the tenants the CALLER is a member of — the data
//behind the org switcher. No arguments: the account comes from the token.
</pre>



<a name="cloud-v1-api-listmytenantsresponse"></a>
### cloud.v1.api.ListMyTenantsResponse

<pre>
//ListMyTenantsResponse returns the caller's tenants (the org switcher data).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenants</td>
<td><a href="../iam/README.md#cloud-v1-iam-tenant">cloud.v1.iam.Tenant</a></td>
<td><pre>
//tenants are the workspaces the caller is a member of.<br>

json_name: tenants
go_name: Tenants</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listpackagesrequest"></a>
### cloud.v1.api.ListPackagesRequest

<pre>
//ListPackagesRequest lists a tenant's packages with filtering, sort and paging.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>db_kinds</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//db_kinds narrows to specific target database engines (facet filter).<br>

json_name: dbKinds
go_name: DbKinds</pre></td>
</tr><tr>
<td>filter</td>
<td><a href="../common/README.md#cloud-v1-common-entityfilter">cloud.v1.common.EntityFilter</a></td>
<td><pre>
//filter is the shared Entity-level filter (search, ids, time windows).<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>formats</td>
<td><a href="../models/README.md#cloud-v1-models-packagerecord-format">cloud.v1.models.PackageRecord.Format</a></td>
<td><pre>
//formats narrows to specific package formats (facet filter).<br>

json_name: formats
go_name: Formats</pre></td>
</tr><tr>
<td>page</td>
<td><a href="../common/README.md#cloud-v1-common-page">cloud.v1.common.Page</a></td>
<td><pre>
//page is the pagination cursor/size.<br>

json_name: page
go_name: Page</pre></td>
</tr><tr>
<td>sort</td>
<td><a href="../common/README.md#cloud-v1-common-entitysort">cloud.v1.common.EntitySort</a></td>
<td><pre>
//sort is the ordering over the common Entity columns.<br>

json_name: sort
go_name: Sort</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the listing to one tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listpackagesresponse"></a>
### cloud.v1.api.ListPackagesResponse

<pre>
//ListPackagesResponse returns one page of packages.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr><tr>
<td>packages</td>
<td><a href="../models/README.md#cloud-v1-models-packagerecord">cloud.v1.models.PackageRecord</a></td>
<td><pre>
//packages is this page of package records.<br>

json_name: packages
go_name: Packages</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listpermissionsrequest"></a>
### cloud.v1.api.ListPermissionsRequest

<pre>
//ListPermissionsRequest takes no arguments. ListPermissions returns the
//catalog of grantable Permissions, assembled by the server from the
//(cloud.v1.iam.auth) annotations across ALL services in the build (not only
//IamAPI), PLUS a synthesized {resource, ACTION_MANAGE} entry for every
//Resource that appears — MANAGE is grantable on a role but never named in an
//annotation, so it must be added explicitly. RESOURCE/ACTION pairs that no
//annotation references (e.g. RESOURCE_ACCOUNT/ACTION_CREATE, which is
//admin_only) are intentionally absent: you cannot grant what nothing checks.
</pre>



<a name="cloud-v1-api-listpermissionsresponse"></a>
### cloud.v1.api.ListPermissionsResponse

<pre>
//ListPermissionsResponse returns the full grantable-permission catalog.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>entries</td>
<td><a href="#cloud-v1-api-catalogentry">cloud.v1.api.CatalogEntry</a></td>
<td><pre>
//entries are the grantable permissions, each with a UI label.<br>

json_name: entries
go_name: Entries</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listquotasrequest"></a>
### cloud.v1.api.ListQuotasRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>refresh_policy</td>
<td><a href="#cloud-v1-api-quotarefreshpolicy">cloud.v1.api.QuotaRefreshPolicy</a></td>
<td><pre>
json_name: refreshPolicy
go_name: RefreshPolicy</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listquotasresponse"></a>
### cloud.v1.api.ListQuotasResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quotas</td>
<td><a href="#cloud-v1-api-quotaview">cloud.v1.api.QuotaView</a></td>
<td><pre>
json_name: quotas
go_name: Quotas</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listrecipesrequest"></a>
### cloud.v1.api.ListRecipesRequest

<pre>
//ListRecipesRequest lists recipe records with filtering and pagination.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>filter</td>
<td><a href="../common/README.md#cloud-v1-common-entityfilter">cloud.v1.common.EntityFilter</a></td>
<td><pre>
//filter holds the shared Entity-level filters (search, ids, time
//windows).<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>page</td>
<td><a href="../common/README.md#cloud-v1-common-page">cloud.v1.common.Page</a></td>
<td><pre>
//page carries pagination (page size + token).<br>

json_name: page
go_name: Page</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listrecipesresponse"></a>
### cloud.v1.api.ListRecipesResponse

<pre>
//ListRecipesResponse returns a page of recipe records.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token fetches the following page; empty when at the end.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr><tr>
<td>recipes</td>
<td><a href="../models/README.md#cloud-v1-models-reciperecord">cloud.v1.models.RecipeRecord</a></td>
<td><pre>
//recipes is the matching page of records.<br>

json_name: recipes
go_name: Recipes</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listregistrationrequestsrequest"></a>
### cloud.v1.api.ListRegistrationRequestsRequest

<pre>
//ListRegistrationRequestsRequest lists requests for the admin console. An
//unspecified status filter returns every request (newest first).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>status</td>
<td><a href="#cloud-v1-api-registrationrequeststatus">cloud.v1.api.RegistrationRequestStatus</a></td>
<td><pre>
status optionally filters by lifecycle state; UNSPECIFIED = all.<br>

json_name: status
go_name: Status</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listregistrationrequestsresponse"></a>
### cloud.v1.api.ListRegistrationRequestsResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>requests</td>
<td><a href="#cloud-v1-api-registrationrequest">cloud.v1.api.RegistrationRequest</a></td>
<td><pre>
json_name: requests
go_name: Requests</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listrolesrequest"></a>
### cloud.v1.api.ListRolesRequest

<pre>
//ListRolesRequest lists roles visible to the caller. tenant_id filters to one
//tenant's roles; empty lists the platform-scoped roles. System roles are
//included and marked via iam.Role.is_system.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>page_size</td>
<td>uint32</td>
<td><pre>
//page_size caps returned rows; 0 -> server default.<br>

json_name: pageSize
go_name: PageSize</pre></td>
</tr><tr>
<td>page_token</td>
<td>string</td>
<td><pre>
//page_token is the opaque cursor from a previous response.<br>

json_name: pageToken
go_name: PageToken</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id filters to one tenant's roles; empty lists platform-scoped roles.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listrolesresponse"></a>
### cloud.v1.api.ListRolesResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr><tr>
<td>roles</td>
<td><a href="../iam/README.md#cloud-v1-iam-role">cloud.v1.iam.Role</a></td>
<td><pre>
//roles is this page of roles (system roles marked via Role.is_system).<br>

json_name: roles
go_name: Roles</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listrunsrequest"></a>
### cloud.v1.api.ListRunsRequest

<pre>
//ListRunsRequest lists the runs launched from recipe bundles for a tenant,
//optionally narrowed to one recipe.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>page</td>
<td><a href="../common/README.md#cloud-v1-common-page">cloud.v1.common.Page</a></td>
<td><pre>
//page carries pagination (page size + token). See ListRunsResponse.next_page_token
//for the important caveat when recipe_id is also set.<br>

json_name: page
go_name: Page</pre></td>
</tr><tr>
<td>recipe_id</td>
<td>string</td>
<td><pre>
//recipe_id, when set, filters to runs launched from that recipe record
//(matches models.Run.recipe_id). Empty returns every recipe
//run for the tenant.<br>

json_name: recipeId
go_name: RecipeId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listrunsresponse"></a>
### cloud.v1.api.ListRunsResponse

<pre>
//ListRunsResponse returns a page of the matching recipe runs.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token fetches the following page; empty when at the end.

//Caveat when ListRunsRequest.recipe_id is set: the recipe_id filter is
//applied in-process over the already-paginated tenant page (see
//RecipeService.ListRuns's doc), so a single page can legitimately
//return zero matching rows while next_page_token is still non-empty.
//Clients filtering by recipe_id MUST keep paginating until
//next_page_token is empty rather than stopping on an empty runs page.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr><tr>
<td>runs</td>
<td><a href="../models/README.md#cloud-v1-models-run">cloud.v1.models.Run</a></td>
<td><pre>
//runs is the matching page of run records.<br>

json_name: runs
go_name: Runs</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listsharesrequest"></a>
### cloud.v1.api.ListSharesRequest

<pre>
//ListSharesRequest lists a tenant's shares with filtering, sort and paging.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>filter</td>
<td><a href="../common/README.md#cloud-v1-common-entityfilter">cloud.v1.common.EntityFilter</a></td>
<td><pre>
//filter is the shared Entity-level filter (search, ids, time windows).<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>page</td>
<td><a href="../common/README.md#cloud-v1-common-page">cloud.v1.common.Page</a></td>
<td><pre>
//page is the pagination cursor/size.<br>

json_name: page
go_name: Page</pre></td>
</tr><tr>
<td>sort</td>
<td><a href="../common/README.md#cloud-v1-common-entitysort">cloud.v1.common.EntitySort</a></td>
<td><pre>
//sort is the ordering over the common Entity columns.<br>

json_name: sort
go_name: Sort</pre></td>
</tr><tr>
<td>target_id</td>
<td>string</td>
<td><pre>
//target_id narrows to shares of one run; empty = any.<br>

json_name: targetId
go_name: TargetId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the listing to one tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-listsharesresponse"></a>
### cloud.v1.api.ListSharesResponse

<pre>
//ListSharesResponse returns one page of shares.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>next_page_token</td>
<td>string</td>
<td><pre>
//next_page_token is empty when there are no more rows.<br>

json_name: nextPageToken
go_name: NextPageToken</pre></td>
</tr><tr>
<td>shares</td>
<td><a href="../models/README.md#cloud-v1-models-sharerecord">cloud.v1.models.ShareRecord</a></td>
<td><pre>
//shares is this page of share records.<br>

json_name: shares
go_name: Shares</pre></td>
</tr>
</table>



<a name="cloud-v1-api-liststroppyversionsrequest"></a>
### cloud.v1.api.ListStroppyVersionsRequest

<pre>
//ListStroppyVersionsRequest lists release tags accepted by the server
//configuration.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the caller's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-liststroppyversionsresponse"></a>
### cloud.v1.api.ListStroppyVersionsResponse

<pre>
//ListStroppyVersionsResponse returns release tags sorted newest first.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>versions</td>
<td>string</td>
<td><pre>
//versions are release tags from the configured GitHub repository, filtered
//by the server's configured minimum version.<br>

json_name: versions
go_name: Versions</pre></td>
</tr>
</table>



<a name="cloud-v1-api-logfacetfield"></a>
### cloud.v1.api.LogFacetField

<pre>
//LogFacetField is the distinct-value set for one log filter dimension.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>field</td>
<td>string</td>
<td><pre>
//field is the wire field name (e.g. component_id, machine_id, unit, phase,
//action, step_id, stage_name, node_execution_id).<br>

json_name: field
go_name: Field</pre></td>
</tr><tr>
<td>values</td>
<td><a href="#cloud-v1-api-logfacetvalue">cloud.v1.api.LogFacetValue</a></td>
<td><pre>
//values are the distinct values with hit counts, most frequent first.<br>

json_name: values
go_name: Values</pre></td>
</tr>
</table>



<a name="cloud-v1-api-logfacetvalue"></a>
### cloud.v1.api.LogFacetValue

<pre>
//LogFacetValue is one distinct value of a facet field plus how many lines carry
//it (under the request filter).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>count</td>
<td>uint64</td>
<td><pre>
json_name: count
go_name: Count</pre></td>
</tr><tr>
<td>value</td>
<td>string</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="cloud-v1-api-logfilter"></a>
### cloud.v1.api.LogFilter

<pre>
//LogFilter is the flexible log filter shared by query and stream. All fields are
//optional and AND-combined. `query` is the raw-LogsQL escape hatch for advanced
//filtering beyond the structured fields.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>actions</td>
<td>string</td>
<td><pre>
//actions restrict to generic action classes such as call_cmd/write_file.<br>

json_name: actions
go_name: Actions</pre></td>
</tr><tr>
<td>component_ids</td>
<td>string</td>
<td><pre>
//component_ids restricts to lines emitted by these components.<br>

json_name: componentIds
go_name: ComponentIds</pre></td>
</tr><tr>
<td>end</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//end keeps lines at or before this time.<br>

json_name: end
go_name: End</pre></td>
</tr><tr>
<td>machine_ids</td>
<td>string</td>
<td><pre>
//machine_ids restricts to logs emitted by these provider/runtime machines.
//This is the explicit UI-facing machine filter; node_ids remains accepted
//for older links and clients.<br>

json_name: machineIds
go_name: MachineIds</pre></td>
</tr><tr>
<td>mentions</td>
<td>string</td>
<td><pre>
//mentions restrict to normalized operation tokens such as vector/vmagent/postgres.<br>

json_name: mentions
go_name: Mentions</pre></td>
</tr><tr>
<td>node_execution_ids</td>
<td>string</td>
<td><pre>
//node_execution_ids restricts to lines emitted by these node executions.<br>

json_name: nodeExecutionIds
go_name: NodeExecutionIds</pre></td>
</tr><tr>
<td>node_ids</td>
<td>string</td>
<td><pre>
//node_ids restricts to lines emitted by these logical topology nodes.<br>

json_name: nodeIds
go_name: NodeIds</pre></td>
</tr><tr>
<td>parent_node_execution_ids</td>
<td>string</td>
<td><pre>
//parent_node_execution_ids restrict to logs under these parent stages.<br>

json_name: parentNodeExecutionIds
go_name: ParentNodeExecutionIds</pre></td>
</tr><tr>
<td>phases</td>
<td>string</td>
<td><pre>
//phases restrict to logs belonging to these top-level pipeline phases.<br>

json_name: phases
go_name: Phases</pre></td>
</tr><tr>
<td>query</td>
<td>string</td>
<td><pre>
//query is a raw LogsQL fragment, AND-ed with the structured filters (advanced).<br>

json_name: query
go_name: Query</pre></td>
</tr><tr>
<td>search</td>
<td>string</td>
<td><pre>
//search is a simple substring match over the line text.<br>

json_name: search
go_name: Search</pre></td>
</tr><tr>
<td>sources</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-source">cloud.v1.monitor.Source</a></td>
<td><pre>
//sources restricts to these log sources.<br>

json_name: sources
go_name: Sources</pre></td>
</tr><tr>
<td>stage_names</td>
<td>string</td>
<td><pre>
//stage_names restrict to logs emitted by stages with these display names.<br>

json_name: stageNames
go_name: StageNames</pre></td>
</tr><tr>
<td>start</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//start keeps lines at or after this time.<br>

json_name: start
go_name: Start</pre></td>
</tr><tr>
<td>step_ids</td>
<td>string</td>
<td><pre>
//step_ids restrict to component-local deployment step ids.<br>

json_name: stepIds
go_name: StepIds</pre></td>
</tr><tr>
<td>streams</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-stream">cloud.v1.monitor.Stream</a></td>
<td><pre>
//streams restricts to these log streams (e.g. stdout/stderr).<br>

json_name: streams
go_name: Streams</pre></td>
</tr><tr>
<td>unit</td>
<td>string</td>
<td><pre>
//unit restricts to a specific systemd/log unit.<br>

json_name: unit
go_name: Unit</pre></td>
</tr><tr>
<td>units</td>
<td>string</td>
<td><pre>
//units restrict to these systemd units or tailed log-file units.
//unit remains accepted for older links and clients.<br>

json_name: units
go_name: Units</pre></td>
</tr>
</table>



<a name="cloud-v1-api-logscrolldirection"></a>
### cloud.v1.api.LogScrollDirection

<pre>
//LogScrollDirection selects which way QueryLogs pages relative to the anchor.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>LOG_SCROLL_DIRECTION_UNSPECIFIED</td>
<td><pre>
//LOG_SCROLL_DIRECTION_UNSPECIFIED uses the server default (newer).
</pre></td>
</tr><tr>
<td>LOG_SCROLL_DIRECTION_OLDER</td>
<td><pre>
//LOG_SCROLL_DIRECTION_OLDER pages back in time.
</pre></td>
</tr><tr>
<td>LOG_SCROLL_DIRECTION_NEWER</td>
<td><pre>
//LOG_SCROLL_DIRECTION_NEWER pages forward in time.
</pre></td>
</tr>
</table>

<a name="cloud-v1-api-loginrequest"></a>
### cloud.v1.api.LoginRequest

<pre>
//LoginRequest authenticates by login + password. login accepts EITHER a
//nickname OR an email; the server resolves which by format. No separate flag:
//"alice" and "alice@example.com" both go in login.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>login</td>
<td>string</td>
<td><pre>
//login is the account's nickname or email.<br>

json_name: login
go_name: Login</pre></td>
</tr><tr>
<td>password</td>
<td>string</td>
<td><pre>
//password is the plaintext password, verified against the stored hash.<br>

json_name: password
go_name: Password</pre></td>
</tr>
</table>



<a name="cloud-v1-api-loginresponse"></a>
### cloud.v1.api.LoginResponse

<pre>
//LoginResponse returns the issued credential pair on a successful Login.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tokens</td>
<td><a href="#cloud-v1-api-tokenpair">cloud.v1.api.TokenPair</a></td>
<td><pre>
//tokens is the freshly minted access + refresh credential pair.<br>

json_name: tokens
go_name: Tokens</pre></td>
</tr>
</table>



<a name="cloud-v1-api-logoutrequest"></a>
### cloud.v1.api.LogoutRequest

<pre>
//LogoutRequest revokes the server-side session for the given refresh_token.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>refresh_token</td>
<td>string</td>
<td><pre>
//refresh_token identifies the server-side session to revoke.<br>

json_name: refreshToken
go_name: RefreshToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-logoutresponse"></a>
### cloud.v1.api.LogoutResponse

<pre>
//LogoutResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-lookupaccountbyemailrequest"></a>
### cloud.v1.api.LookupAccountByEmailRequest

<pre>
//LookupAccountByEmailRequest resolves an EXACT email to its account — the
//invite-by-email primitive. Any authenticated caller may use it (e.g. a
//tenant owner adding a member) without holding RESOURCE_ACCOUNT/LIST, so it
//deliberately exposes no listing/enumeration: an exact hit or NotFound.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>email</td>
<td>string</td>
<td><pre>
json_name: email
go_name: Email</pre></td>
</tr>
</table>



<a name="cloud-v1-api-lookupaccountbyemailresponse"></a>
### cloud.v1.api.LookupAccountByEmailResponse

<pre>
//LookupAccountByEmailResponse returns the matched account (no secrets).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account</td>
<td><a href="../iam/README.md#cloud-v1-iam-account">cloud.v1.iam.Account</a></td>
<td><pre>
json_name: account
go_name: Account</pre></td>
</tr>
</table>



<a name="cloud-v1-api-markregistrationrequesthandledrequest"></a>
### cloud.v1.api.MarkRegistrationRequestHandledRequest

<pre>
//MarkRegistrationRequestHandledRequest flips one request to HANDLED. Account
//creation is a separate admin action (CreateAccount) — this only triages.
//Idempotent: marking an already-handled request is a no-op.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-markregistrationrequesthandledresponse"></a>
### cloud.v1.api.MarkRegistrationRequestHandledResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>request</td>
<td><a href="#cloud-v1-api-registrationrequest">cloud.v1.api.RegistrationRequest</a></td>
<td><pre>
json_name: request
go_name: Request</pre></td>
</tr>
</table>



<a name="cloud-v1-api-platformsettings"></a>
### cloud.v1.api.PlatformSettings

<pre>
//PlatformSettings is the GLOBAL (singleton) control-plane configuration, changed
//only by the root admin. It is NOT tenant-scoped — there is one control plane and
//one row.

//server_addr is the single public control-plane base URL handed to every agent
//(STROPPY_SERVER_ADDR) so it knows where to Poll/Report and fetch its binary (the
//agent binary URL is DERIVED from server_addr + the server's agent-binary
//endpoint — no separate setting). Empty -> the server derives a docker-host
//fallback (host.docker.internal / bridge gateway) for local Docker runs.

//PROCESS GATES (allow_* flags). The remaining fields are coarse, global
//kill-switches that the service layer checks BEFORE any RBAC permission, to
//open or close whole flows platform-wide. They sit ABOVE the role model: a
//closed gate denies everyone EXCEPT platform admins (Account.is_admin), who
//are never blocked by these flags. RBAC still applies on top once a gate is
//open. Every flag is polarised so its zero value (false) is the SAFE, most
//restrictive state — a fresh/empty PlatformSettings locks the platform down.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>allow_member_tenant_creation</td>
<td>bool</td>
<td><pre>
//allow_member_tenant_creation governs who may create tenants. When false
//(default) only platform admins may call CreateTenant, regardless of any
//RESOURCE_TENANT/ACTION_CREATE grant. When true, the normal RBAC permission
//check applies and ordinary members can spin up tenants.<br>

json_name: allowMemberTenantCreation
go_name: AllowMemberTenantCreation</pre></td>
</tr><tr>
<td>allow_self_registration</td>
<td>bool</td>
<td><pre>
//allow_self_registration toggles the PUBLIC self-signup endpoint
//(IamAPI.Register). When false (default) there is no open registration:
//accounts can only be created by a platform admin via CreateAccount. Flip
//it true to let anyone sign themselves up.<br>

json_name: allowSelfRegistration
go_name: AllowSelfRegistration</pre></td>
</tr><tr>
<td>server_addr</td>
<td>string</td>
<td><pre>
//server_addr is the public control-plane base URL, e.g. http://1.1.1.1:8080.<br>

json_name: serverAddr
go_name: ServerAddr</pre></td>
</tr>
</table>



<a name="cloud-v1-api-publicratingentry"></a>
### cloud.v1.api.PublicRatingEntry

<pre>
//PublicRatingEntry is one ranked benchmark, sensitive fields stripped.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>db_kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//db_kind is the database engine the benchmark ran against.<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>metric_unit</td>
<td>string</td>
<td><pre>
//metric_unit is the unit the metric_value is expressed in.<br>

json_name: metricUnit
go_name: MetricUnit</pre></td>
</tr><tr>
<td>metric_value</td>
<td>double</td>
<td><pre>
//metric_value is the ranked metric's value for this entry.<br>

json_name: metricValue
go_name: MetricValue</pre></td>
</tr><tr>
<td>node_count</td>
<td>uint32</td>
<td><pre>
//node_count is the number of nodes in the topology.<br>

json_name: nodeCount
go_name: NodeCount</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the deployment/cloud provider the benchmark ran on.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>rank</td>
<td>uint32</td>
<td><pre>
//rank is the 1-based position on the leaderboard.<br>

json_name: rank
go_name: Rank</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
//stroppy_version is the stroppy engine version used.<br>

json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr><tr>
<td>topology_label</td>
<td>string</td>
<td><pre>
//topology_label is a human-readable summary of the cluster topology.<br>

json_name: topologyLabel
go_name: TopologyLabel</pre></td>
</tr><tr>
<td>workload_name</td>
<td>string</td>
<td><pre>
//workload_name is the workload the benchmark executed.<br>

json_name: workloadName
go_name: WorkloadName</pre></td>
</tr>
</table>



<a name="cloud-v1-api-querylogsrequest"></a>
### cloud.v1.api.QueryLogsRequest

<pre>
//QueryLogsRequest fetches a bounded, cursor-paged window of historical log lines.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>direction</td>
<td><a href="#cloud-v1-api-logscrolldirection">cloud.v1.api.LogScrollDirection</a></td>
<td><pre>
//direction selects which way to page relative to the anchor.<br>

json_name: direction
go_name: Direction</pre></td>
</tr><tr>
<td>filter</td>
<td><a href="#cloud-v1-api-logfilter">cloud.v1.api.LogFilter</a></td>
<td><pre>
//filter narrows the lines returned.<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>from</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logcursor">cloud.v1.monitor.LogCursor</a></td>
<td><pre>
//from is the anchor to page from; empty = newest (when OLDER) / oldest (when
//NEWER).<br>

json_name: from
go_name: From</pre></td>
</tr><tr>
<td>limit</td>
<td>uint32</td>
<td><pre>
//limit caps the lines returned; 0 -> server default. Bounded so the UI never
//drowns.<br>

json_name: limit
go_name: Limit</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id identifies the run whose logs are queried.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-querylogsresponse"></a>
### cloud.v1.api.QueryLogsResponse

<pre>
//QueryLogsResponse returns a page of log lines plus cursors to page either way.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>lines</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logline">cloud.v1.monitor.LogLine</a></td>
<td><pre>
//lines is the matching page of log lines.<br>

json_name: lines
go_name: Lines</pre></td>
</tr><tr>
<td>newer</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logcursor">cloud.v1.monitor.LogCursor</a></td>
<td><pre>
//newer is the cursor to fetch the page NEWER than these results (empty = at
//the end / tip).<br>

json_name: newer
go_name: Newer</pre></td>
</tr><tr>
<td>older</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logcursor">cloud.v1.monitor.LogCursor</a></td>
<td><pre>
//older is the cursor to fetch the page OLDER than these results (empty = at
//the start).<br>

json_name: older
go_name: Older</pre></td>
</tr>
</table>



<a name="cloud-v1-api-quotarefreshpolicy"></a>
### cloud.v1.api.QuotaRefreshPolicy

<pre>
//QuotaRefreshPolicy controls whether a read may talk to the provider.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>QUOTA_REFRESH_POLICY_UNSPECIFIED</td>
<td><pre>
//QUOTA_REFRESH_POLICY_UNSPECIFIED behaves as REFRESH_IF_STALE.
</pre></td>
</tr><tr>
<td>QUOTA_REFRESH_POLICY_CACHE_ONLY</td>
<td><pre>
//QUOTA_REFRESH_POLICY_CACHE_ONLY returns stored snapshots only.
</pre></td>
</tr><tr>
<td>QUOTA_REFRESH_POLICY_REFRESH_IF_STALE</td>
<td><pre>
//QUOTA_REFRESH_POLICY_REFRESH_IF_STALE refreshes stale/missing snapshots.
</pre></td>
</tr><tr>
<td>QUOTA_REFRESH_POLICY_FORCE_REFRESH</td>
<td><pre>
//QUOTA_REFRESH_POLICY_FORCE_REFRESH always refreshes from the provider.
</pre></td>
</tr>
</table>

<a name="cloud-v1-api-quotareservationview"></a>
### cloud.v1.api.QuotaReservationView

<pre>
//QuotaReservationView is one run reservation/allocation row.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>amount</td>
<td>uint64</td>
<td><pre>
json_name: amount
go_name: Amount</pre></td>
</tr><tr>
<td>created_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>expires_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
json_name: expiresAt
go_name: ExpiresAt</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>info</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-info">cloud.v1.deployment.Quota.Info</a></td>
<td><pre>
json_name: info
go_name: Info</pre></td>
</tr><tr>
<td>node_id</td>
<td>string</td>
<td><pre>
json_name: nodeId
go_name: NodeId</pre></td>
</tr><tr>
<td>resource_id</td>
<td>string</td>
<td><pre>
json_name: resourceId
go_name: ResourceId</pre></td>
</tr><tr>
<td>resource_type</td>
<td>string</td>
<td><pre>
json_name: resourceType
go_name: ResourceType</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>service</td>
<td>string</td>
<td><pre>
json_name: service
go_name: Service</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-reservationstatus">cloud.v1.deployment.Quota.ReservationStatus</a></td>
<td><pre>
json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-api-quotaview"></a>
### cloud.v1.api.QuotaView

<pre>
//QuotaView is one quota row as shown to users/operators.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>available_for_runs</td>
<td>double</td>
<td><pre>
//available_for_runs is max(provider_available - reserved, 0).<br>

json_name: availableForRuns
go_name: AvailableForRuns</pre></td>
</tr><tr>
<td>info</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-quota-info">cloud.v1.deployment.Quota.Info</a></td>
<td><pre>
//info identifies the provider quota.<br>

json_name: info
go_name: Info</pre></td>
</tr><tr>
<td>limit</td>
<td>double</td>
<td><pre>
//limit is the provider quota limit.<br>

json_name: limit
go_name: Limit</pre></td>
</tr><tr>
<td>observed_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//observed_at is when the provider snapshot was fetched.<br>

json_name: observedAt
go_name: ObservedAt</pre></td>
</tr><tr>
<td>provider_available</td>
<td>double</td>
<td><pre>
//provider_available is max(limit - provider_used, 0).<br>

json_name: providerAvailable
go_name: ProviderAvailable</pre></td>
</tr><tr>
<td>provider_used</td>
<td>double</td>
<td><pre>
//provider_used is the usage value reported by the provider snapshot.<br>

json_name: providerUsed
go_name: ProviderUsed</pre></td>
</tr><tr>
<td>reserved</td>
<td>double</td>
<td><pre>
//reserved is our active pre-deploy reservation amount.<br>

json_name: reserved
go_name: Reserved</pre></td>
</tr><tr>
<td>resource_id</td>
<td>string</td>
<td><pre>
//resource_id is the provider scope id, e.g. YC cloud_id.<br>

json_name: resourceId
go_name: ResourceId</pre></td>
</tr><tr>
<td>resource_type</td>
<td>string</td>
<td><pre>
//resource_type is the provider scope type, e.g. resource-manager.cloud.<br>

json_name: resourceType
go_name: ResourceType</pre></td>
</tr><tr>
<td>service</td>
<td>string</td>
<td><pre>
//service is the provider service id, e.g. compute.<br>

json_name: service
go_name: Service</pre></td>
</tr><tr>
<td>stale</td>
<td>bool</td>
<td><pre>
//stale is true when now >= stale_after or the row is synthetic.<br>

json_name: stale
go_name: Stale</pre></td>
</tr><tr>
<td>stale_after</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//stale_after is when the snapshot should be refreshed.<br>

json_name: staleAfter
go_name: StaleAfter</pre></td>
</tr>
</table>



<a name="cloud-v1-api-ratingentry"></a>
### cloud.v1.api.RatingEntry

<pre>
//RatingEntry is one ranked benchmark for the authenticated boards. Includes
//identifying info (run_id, author, tenant) the public board omits.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>author_name</td>
<td>string</td>
<td><pre>
//author_name is who created the run (authenticated scopes only).<br>

json_name: authorName
go_name: AuthorName</pre></td>
</tr><tr>
<td>db_kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//db_kind is the database engine the benchmark ran against.<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>metric_unit</td>
<td>string</td>
<td><pre>
//metric_unit is the unit the metric_value is expressed in.<br>

json_name: metricUnit
go_name: MetricUnit</pre></td>
</tr><tr>
<td>metric_value</td>
<td>double</td>
<td><pre>
//metric_value is the ranked metric's value for this entry.<br>

json_name: metricValue
go_name: MetricValue</pre></td>
</tr><tr>
<td>node_count</td>
<td>uint32</td>
<td><pre>
//node_count is the number of nodes in the topology.<br>

json_name: nodeCount
go_name: NodeCount</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the deployment/cloud provider the benchmark ran on.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>rank</td>
<td>uint32</td>
<td><pre>
//rank is the 1-based position on the leaderboard.<br>

json_name: rank
go_name: Rank</pre></td>
</tr><tr>
<td>run_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//run_at is when the benchmark run started.<br>

json_name: runAt
go_name: RunAt</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id identifies the underlying test run (authenticated scopes only).<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
//stroppy_version is the stroppy engine version used.<br>

json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr><tr>
<td>tenant_name</td>
<td>string</td>
<td><pre>
//tenant_name is the owning tenant; set only on the system-wide board.<br>

json_name: tenantName
go_name: TenantName</pre></td>
</tr><tr>
<td>topology_label</td>
<td>string</td>
<td><pre>
//topology_label is a human-readable summary of the cluster topology.<br>

json_name: topologyLabel
go_name: TopologyLabel</pre></td>
</tr><tr>
<td>workload_name</td>
<td>string</td>
<td><pre>
//workload_name is the workload the benchmark executed.<br>

json_name: workloadName
go_name: WorkloadName</pre></td>
</tr>
</table>



<a name="cloud-v1-api-ratingfilter"></a>
### cloud.v1.api.RatingFilter

<pre>
//RatingFilter selects and ranks the benchmark runs. Ranking is by one metric
//(metric_key); direction is the metric's own higher_is_better.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>db_kinds</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//db_kinds narrows to specific database engines (AND; empty = not applied).<br>

json_name: dbKinds
go_name: DbKinds</pre></td>
</tr><tr>
<td>metric_key</td>
<td>string</td>
<td><pre>
//metric_key is the metric to rank by (a MetricSummary.key). Required.<br>

json_name: metricKey
go_name: MetricKey</pre></td>
</tr><tr>
<td>providers</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//providers narrows to specific deployment providers (AND; empty = not
//applied).<br>

json_name: providers
go_name: Providers</pre></td>
</tr><tr>
<td>started_after</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_after keeps only runs that started at/after this time.<br>

json_name: startedAfter
go_name: StartedAfter</pre></td>
</tr><tr>
<td>started_before</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_before keeps only runs that started at/before this time.<br>

json_name: startedBefore
go_name: StartedBefore</pre></td>
</tr><tr>
<td>stroppy_versions</td>
<td>string</td>
<td><pre>
//stroppy_versions narrows to specific engine versions (AND; empty = not
//applied).<br>

json_name: stroppyVersions
go_name: StroppyVersions</pre></td>
</tr>
</table>



<a name="cloud-v1-api-refreshquotasrequest"></a>
### cloud.v1.api.RefreshQuotasRequest

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-refreshquotasresponse"></a>
### cloud.v1.api.RefreshQuotasResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>quotas</td>
<td><a href="#cloud-v1-api-quotaview">cloud.v1.api.QuotaView</a></td>
<td><pre>
json_name: quotas
go_name: Quotas</pre></td>
</tr>
</table>



<a name="cloud-v1-api-refreshrequest"></a>
### cloud.v1.api.RefreshRequest

<pre>
//RefreshRequest exchanges a valid refresh_token for a fresh TokenPair. The
//presented refresh_token is consumed (single-use); the response carries a new
//rotated refresh_token. Travels in the body, not a cookie.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>refresh_token</td>
<td>string</td>
<td><pre>
//refresh_token is the single-use token to exchange; consumed on success.<br>

json_name: refreshToken
go_name: RefreshToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-refreshresponse"></a>
### cloud.v1.api.RefreshResponse

<pre>
//RefreshResponse returns the rotated credential pair (new refresh_token).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tokens</td>
<td><a href="#cloud-v1-api-tokenpair">cloud.v1.api.TokenPair</a></td>
<td><pre>
//tokens is the new pair; persist the new refresh_token for the next Refresh.<br>

json_name: tokens
go_name: Tokens</pre></td>
</tr>
</table>



<a name="cloud-v1-api-registerrequest"></a>
### cloud.v1.api.RegisterRequest

<pre>
//RegisterRequest is PUBLIC self-signup. It is gated by
//PlatformSettings.allow_self_registration: when that flag is false the server
//rejects this call and accounts can only be created by an admin via
//CreateAccount. There is deliberately no is_admin field — a self-registered
//account is never a platform admin.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>email</td>
<td>string</td>
<td><pre>
//email is the new account's contact + login address (format-validated).<br>

json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>nickname</td>
<td>string</td>
<td><pre>
//nickname is the URL/handle-safe display handle (also an alternate login).<br>

json_name: nickname
go_name: Nickname</pre></td>
</tr><tr>
<td>password</td>
<td>string</td>
<td><pre>
//password is the plaintext password to set; hashed and stored server-side.<br>

json_name: password
go_name: Password</pre></td>
</tr>
</table>



<a name="cloud-v1-api-registerresponse"></a>
### cloud.v1.api.RegisterResponse

<pre>
//RegisterResponse auto-logs-in the new account by returning a TokenPair, so a
//successful signup needs no follow-up Login. The account's email starts
//UNVERIFIED (Account.email_verified == false); the server dispatches a
//verification token through its Notifier (see VerifyEmailRequest). Login is not
//blocked on verification — it is surfaced for the UI to nudge the user.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tokens</td>
<td><a href="#cloud-v1-api-tokenpair">cloud.v1.api.TokenPair</a></td>
<td><pre>
//tokens auto-logs-in the new account (no follow-up Login needed).<br>

json_name: tokens
go_name: Tokens</pre></td>
</tr>
</table>



<a name="cloud-v1-api-registrationrequest"></a>
### cloud.v1.api.RegistrationRequest

<pre>
//RegistrationRequest is one prospective user's access request, captured while
//self-signup is closed. email is the unique key — a re-submit from the same
//address refreshes the message and keeps a single PENDING row.
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
json_name: createdAt
go_name: CreatedAt</pre></td>
</tr><tr>
<td>email</td>
<td>string</td>
<td><pre>
email is the requester's contact + intended login address.<br>

json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>handled_by_account_id</td>
<td>string</td>
<td><pre>
handled_by_account_id is the admin who marked it handled (empty until then).<br>

json_name: handledByAccountId
go_name: HandledByAccountId</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
id is the server-assigned identifier.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>message</td>
<td>string</td>
<td><pre>
message is the requester's optional free-text note (reason / context).<br>

json_name: message
go_name: Message</pre></td>
</tr><tr>
<td>status</td>
<td><a href="#cloud-v1-api-registrationrequeststatus">cloud.v1.api.RegistrationRequestStatus</a></td>
<td><pre>
status is the triage lifecycle state.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>updated_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
json_name: updatedAt
go_name: UpdatedAt</pre></td>
</tr>
</table>



<a name="cloud-v1-api-registrationrequeststatus"></a>
### cloud.v1.api.RegistrationRequestStatus

<pre>
//RegistrationRequestStatus is the lifecycle of one access request.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>REGISTRATION_REQUEST_STATUS_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>REGISTRATION_REQUEST_STATUS_PENDING</td>
<td><pre>
PENDING: awaiting an admin's attention.
</pre></td>
</tr><tr>
<td>REGISTRATION_REQUEST_STATUS_HANDLED</td>
<td><pre>
HANDLED: an admin has actioned it (account created or dismissed).
</pre></td>
</tr>
</table>

<a name="cloud-v1-api-removefavoriterequest"></a>
### cloud.v1.api.RemoveFavoriteRequest

<pre>
//RemoveFavorite unmarks (kind, target_id). Idempotent: removing an absent
//favorite is a no-op.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>kind</td>
<td><a href="../common/README.md#cloud-v1-common-favoritekind">cloud.v1.common.FavoriteKind</a></td>
<td><pre>
//kind is the favoritable resource type (must be a defined, non-zero kind).<br>

json_name: kind
go_name: Kind</pre></td>
</tr><tr>
<td>target_id</td>
<td>string</td>
<td><pre>
//target_id is the id of the row to unfavorite, within `kind`.<br>

json_name: targetId
go_name: TargetId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the favorite to the caller's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-removefavoriteresponse"></a>
### cloud.v1.api.RemoveFavoriteResponse

<pre>
//RemoveFavoriteResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-requestpasswordresetrequest"></a>
### cloud.v1.api.RequestPasswordResetRequest

<pre>
//RequestPasswordResetRequest starts the PUBLIC forgot-password flow: the
//server mints a single-use, time-bounded reset token and delivers it to the
//account's email via its Notifier. The response is ALWAYS empty/success
//regardless of whether the email exists — never leak account existence.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>email</td>
<td>string</td>
<td><pre>
//email is the address to send the reset token to (existence never leaked).<br>

json_name: email
go_name: Email</pre></td>
</tr>
</table>



<a name="cloud-v1-api-requestpasswordresetresponse"></a>
### cloud.v1.api.RequestPasswordResetResponse

<pre>
//RequestPasswordResetResponse is always empty/success (no account-existence
//leak).
</pre>



<a name="cloud-v1-api-resendverificationrequest"></a>
### cloud.v1.api.ResendVerificationRequest

<pre>
//ResendVerificationRequest re-dispatches a fresh verification token to the
//CALLER's own (still-unverified) email via the Notifier. Authenticated; no
//arguments — the account comes from the token.
</pre>



<a name="cloud-v1-api-resendverificationresponse"></a>
### cloud.v1.api.ResendVerificationResponse

<pre>
//ResendVerificationResponse is empty; success is signalled by the absence of
//error.
</pre>



<a name="cloud-v1-api-resetpasswordrequest"></a>
### cloud.v1.api.ResetPasswordRequest

<pre>
//ResetPasswordRequest is the admin override: a platform admin sets a new
//password for any account without knowing the old one (e.g. account recovery).
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
//account_id is the account whose password the admin is resetting.<br>

json_name: accountId
go_name: AccountId</pre></td>
</tr><tr>
<td>new_password</td>
<td>string</td>
<td><pre>
//new_password is the replacement password to set.<br>

json_name: newPassword
go_name: NewPassword</pre></td>
</tr>
</table>



<a name="cloud-v1-api-resetpasswordresponse"></a>
### cloud.v1.api.ResetPasswordResponse

<pre>
//ResetPasswordResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-resolvelogrefrequest"></a>
### cloud.v1.api.ResolveLogRefRequest

<pre>
//ResolveLogRef turns a shareable LogRef (deep-link, e.g. built from a pipeline
//node) into a concrete filter + anchor cursor, so opening a link lands every
//user on the exact same place.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>ref</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logref">cloud.v1.monitor.LogRef</a></td>
<td><pre>
//ref is the shareable log reference (deep-link) to resolve.<br>

json_name: ref
go_name: Ref</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-resolvelogrefresponse"></a>
### cloud.v1.api.ResolveLogRefResponse

<pre>
//ResolveLogRefResponse returns the run id, filter and anchor cursor the LogRef
//resolves to.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cursor</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logcursor">cloud.v1.monitor.LogCursor</a></td>
<td><pre>
//cursor is the anchor cursor to land on.<br>

json_name: cursor
go_name: Cursor</pre></td>
</tr><tr>
<td>filter</td>
<td><a href="#cloud-v1-api-logfilter">cloud.v1.api.LogFilter</a></td>
<td><pre>
//filter is the concrete filter the ref resolves to.<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the run the ref points at.<br>

json_name: runId
go_name: RunId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-revokeapitokenrequest"></a>
### cloud.v1.api.RevokeApiTokenRequest

<pre>
//RevokeApiTokenRequest permanently disables one token by id. The caller must
//own the token's account or be a platform admin (enforced server-side).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the token to permanently disable.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-revokeapitokenresponse"></a>
### cloud.v1.api.RevokeApiTokenResponse

<pre>
//RevokeApiTokenResponse is empty; success is signalled by the absence of error.
</pre>



<a name="cloud-v1-api-revokesharerequest"></a>
### cloud.v1.api.RevokeShareRequest

<pre>
//RevokeShare disables a share (the public endpoint returns gone) without
//deleting the record. Idempotent.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the share to disable.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the share's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-revokeshareresponse"></a>
### cloud.v1.api.RevokeShareResponse

<pre>
//RevokeShareResponse returns the share after it was disabled.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>share</td>
<td><a href="../models/README.md#cloud-v1-models-sharerecord">cloud.v1.models.ShareRecord</a></td>
<td><pre>
//share is the now-revoked share record.<br>

json_name: share
go_name: Share</pre></td>
</tr>
</table>



<a name="cloud-v1-api-runcolumn"></a>
### cloud.v1.api.RunColumn

<pre>
//RunColumn is one run's descriptor in the comparison (config, not metrics).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>db_kind</td>
<td><a href="../domain/README.md#cloud-v1-domain-database-kind">cloud.v1.domain.Database.Kind</a></td>
<td><pre>
//db_kind is the database engine the run targeted.<br>

json_name: dbKind
go_name: DbKind</pre></td>
</tr><tr>
<td>db_name</td>
<td>string</td>
<td><pre>
//db_name is the database/preset name used by the run.<br>

json_name: dbName
go_name: DbName</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//duration is how long the run took.<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the run's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>node_count</td>
<td>uint32</td>
<td><pre>
//node_count is the number of nodes in the run's topology.<br>

json_name: nodeCount
go_name: NodeCount</pre></td>
</tr><tr>
<td>provider</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-provider">cloud.v1.deployment.Provider</a></td>
<td><pre>
//provider is the deployment/cloud provider the run ran on.<br>

json_name: provider
go_name: Provider</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the test run this column describes.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>started_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//started_at is when the run began (server clock).<br>

json_name: startedAt
go_name: StartedAt</pre></td>
</tr><tr>
<td>status</td>
<td><a href="../common/README.md#cloud-v1-common-status">cloud.v1.common.Status</a></td>
<td><pre>
//status is the run's lifecycle/terminal status.<br>

json_name: status
go_name: Status</pre></td>
</tr><tr>
<td>stroppy_version</td>
<td>string</td>
<td><pre>
//stroppy_version is the stroppy engine version used.<br>

json_name: stroppyVersion
go_name: StroppyVersion</pre></td>
</tr><tr>
<td>topology_label</td>
<td>string</td>
<td><pre>
//topology_label is a human-readable summary of the cluster topology.<br>

json_name: topologyLabel
go_name: TopologyLabel</pre></td>
</tr><tr>
<td>workload_name</td>
<td>string</td>
<td><pre>
//workload_name is the workload/preset the run executed.<br>

json_name: workloadName
go_name: WorkloadName</pre></td>
</tr>
</table>



<a name="cloud-v1-api-setshareexpiryrequest"></a>
### cloud.v1.api.SetShareExpiryRequest

<pre>
//SetShareExpiry changes the lifetime (extend / shorten). Same ttl semantics as
//create (0/unset = default, explicit 0 = never with a warning).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the share whose lifetime to change.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the share's tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr><tr>
<td>ttl</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//ttl is the new lifetime; same semantics as create (0/unset = default,
//explicit 0 = never with a warning).<br>

json_name: ttl
go_name: Ttl</pre></td>
</tr>
</table>



<a name="cloud-v1-api-setshareexpiryresponse"></a>
### cloud.v1.api.SetShareExpiryResponse

<pre>
//SetShareExpiryResponse returns the share after its lifetime was changed.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>share</td>
<td><a href="../models/README.md#cloud-v1-models-sharerecord">cloud.v1.models.ShareRecord</a></td>
<td><pre>
//share is the share record with the updated expiry.<br>

json_name: share
go_name: Share</pre></td>
</tr>
</table>



<a name="cloud-v1-api-settenantprovidersettingsrequest"></a>
### cloud.v1.api.SetTenantProviderSettingsRequest

<pre>
//SetTenantProviderSettings sets/replaces the config for ONE provider. The
//provider is selected by the ProviderSettings oneof variant. This does not
//change TenantSettingsRecord.default_provider; default_provider is a separate
//tenant default used by wizards.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="../deployment/README.md#cloud-v1-deployment-providersettings">cloud.v1.deployment.ProviderSettings</a></td>
<td><pre>
//settings carries one provider-specific settings message.<br>

json_name: settings
go_name: Settings</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-shellclientframe"></a>
### cloud.v1.api.ShellClientFrame

<pre>
//ShellClientFrame is admin -> server. The first frame MUST be `start`.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>close</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-empty">google.protobuf.Empty</a></td>
<td><pre>
//close requests an orderly shutdown of the session.<br>

json_name: close
go_name: Close</pre></td>
</tr><tr>
<td>resize</td>
<td><a href="../agent/README.md#cloud-v1-agent-shellresize">cloud.v1.agent.ShellResize</a></td>
<td><pre>
//resize updates the terminal dimensions mid-session.<br>

json_name: resize
go_name: Resize</pre></td>
</tr><tr>
<td>start</td>
<td><a href="#cloud-v1-api-shellstart">cloud.v1.api.ShellStart</a></td>
<td><pre>
//start opens the session (required first frame).<br>

json_name: start
go_name: Start</pre></td>
</tr><tr>
<td>stdin</td>
<td>bytes</td>
<td><pre>
//stdin carries raw keystrokes typed into the terminal.<br>

json_name: stdin
go_name: Stdin</pre></td>
</tr>
</table>



<a name="cloud-v1-api-shellserverframe"></a>
### cloud.v1.api.ShellServerFrame

<pre>
//ShellServerFrame is server -> admin.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>exit</td>
<td><a href="../agent/README.md#cloud-v1-agent-shellexit">cloud.v1.agent.ShellExit</a></td>
<td><pre>
//exit signals the shell process terminated (with its exit status).<br>

json_name: exit
go_name: Exit</pre></td>
</tr><tr>
<td>stderr</td>
<td>bytes</td>
<td><pre>
//stderr carries the terminal's standard-error bytes.<br>

json_name: stderr
go_name: Stderr</pre></td>
</tr><tr>
<td>stdout</td>
<td>bytes</td>
<td><pre>
//stdout carries the terminal's standard-output bytes.<br>

json_name: stdout
go_name: Stdout</pre></td>
</tr>
</table>



<a name="cloud-v1-api-shellstart"></a>
### cloud.v1.api.ShellStart

<pre>
//ShellStart is the REQUIRED first client frame: it picks the target and opens.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cols</td>
<td>uint32</td>
<td><pre>
//cols is the initial terminal width in columns.<br>

json_name: cols
go_name: Cols</pre></td>
</tr><tr>
<td>component_id</td>
<td>string</td>
<td><pre>
//component_id is an optional target component on the host.<br>

json_name: componentId
go_name: ComponentId</pre></td>
</tr><tr>
<td>machine_id</td>
<td>string</td>
<td><pre>
//machine_id is the target host. Required.<br>

json_name: machineId
go_name: MachineId</pre></td>
</tr><tr>
<td>rows</td>
<td>uint32</td>
<td><pre>
//rows is the initial terminal height in rows.<br>

json_name: rows
go_name: Rows</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id is the run context for audit/scoping (optional but recommended).<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>shell</td>
<td>string</td>
<td><pre>
//shell is the shell to launch; empty -> agent default.<br>

json_name: shell
go_name: Shell</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the session to one tenant; the streaming auth
//interceptor reads it from this first frame.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-ssobutton"></a>
### cloud.v1.api.SsoButton

<pre>
//SsoButton is the public, login-page view of an enabled provider.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>display_name</td>
<td>string</td>
<td><pre>
//display_name is the button label.<br>

json_name: displayName
go_name: DisplayName</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the provider id to pass to StartSSO.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
//slug is the provider's url-safe key.<br>

json_name: slug
go_name: Slug</pre></td>
</tr>
</table>



<a name="cloud-v1-api-startrunrequest"></a>
### cloud.v1.api.StartRunRequest

<pre>
//StartRunRequest launches a new run of an already-stored recipe bundle.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>recipe_id</td>
<td>string</td>
<td><pre>
//recipe_id is the recipe record whose stored bundle is launched.<br>

json_name: recipeId
go_name: RecipeId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-startrunresponse"></a>
### cloud.v1.api.StartRunResponse

<pre>
//StartRunResponse returns the persisted, launched run record. The run
//reuses models.Run so overview/metrics/logs work unchanged for
//a recipe run; its spec/topology fields are left empty (a recipe run has
//no baked domain.TestRun — its input is the recipe bundle, carried by the
//launched RunRecipeWorkflow instead).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run</td>
<td><a href="../models/README.md#cloud-v1-models-run">cloud.v1.models.Run</a></td>
<td><pre>
//run is the persisted, launched run record.<br>

json_name: run
go_name: Run</pre></td>
</tr>
</table>



<a name="cloud-v1-api-startssorequest"></a>
### cloud.v1.api.StartSSORequest

<pre>
//StartSSORequest begins the OIDC authorization-code flow for one provider. The
//server builds the IdP authorize URL (with a freshly generated state + PKCE
//challenge it stashes server-side) and returns it; the client redirects the
//user there.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider_id</td>
<td>string</td>
<td><pre>
//provider_id is the OIDC provider to begin the authorization-code flow for.<br>

json_name: providerId
go_name: ProviderId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-startssoresponse"></a>
### cloud.v1.api.StartSSOResponse

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>redirect_url</td>
<td>string</td>
<td><pre>
//redirect_url is the IdP authorize endpoint the caller must send the user
//to.<br>

json_name: redirectUrl
go_name: RedirectUrl</pre></td>
</tr><tr>
<td>state</td>
<td>string</td>
<td><pre>
//state is the opaque CSRF token echoed back to CompleteSSO; the server also
//keeps it to validate the callback.<br>

json_name: state
go_name: State</pre></td>
</tr>
</table>



<a name="cloud-v1-api-statuscounts"></a>
### cloud.v1.api.StatusCounts

<pre>
//StatusCounts is the run breakdown by lifecycle status.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cancelled</td>
<td>uint32</td>
<td><pre>
//cancelled is the count of runs that were cancelled.<br>

json_name: cancelled
go_name: Cancelled</pre></td>
</tr><tr>
<td>completed</td>
<td>uint32</td>
<td><pre>
//completed is the count of successfully finished runs.<br>

json_name: completed
go_name: Completed</pre></td>
</tr><tr>
<td>failed</td>
<td>uint32</td>
<td><pre>
//failed is the count of runs that ended in failure.<br>

json_name: failed
go_name: Failed</pre></td>
</tr><tr>
<td>pending</td>
<td>uint32</td>
<td><pre>
//pending is the count of runs not yet started.<br>

json_name: pending
go_name: Pending</pre></td>
</tr><tr>
<td>running</td>
<td>uint32</td>
<td><pre>
//running is the count of currently executing runs.<br>

json_name: running
go_name: Running</pre></td>
</tr><tr>
<td>total</td>
<td>uint32</td>
<td><pre>
//total is the count of all runs in scope.<br>

json_name: total
go_name: Total</pre></td>
</tr>
</table>



<a name="cloud-v1-api-streamlogsrequest"></a>
### cloud.v1.api.StreamLogsRequest

<pre>
//StreamLogsRequest opens a live log tail. StreamLogs returns a live stream of
//LogLine; the server MAY coalesce / rate-limit under very high throughput. The
//client uses QueryLogs for exact scrollback.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>filter</td>
<td><a href="#cloud-v1-api-logfilter">cloud.v1.api.LogFilter</a></td>
<td><pre>
//filter is the live filter (the user can re-subscribe with a new filter to
//filter the tail).<br>

json_name: filter
go_name: Filter</pre></td>
</tr><tr>
<td>from</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-logcursor">cloud.v1.monitor.LogCursor</a></td>
<td><pre>
//from is an optional start position; empty = from now (tail). Set to backfill
//from a point.<br>

json_name: from
go_name: From</pre></td>
</tr><tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id identifies the run to tail.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-streamtestrunoverviewrequest"></a>
### cloud.v1.api.StreamTestRunOverviewRequest

<pre>
//StreamTestRunOverviewRequest opens a live overview stream for a run. Each stream
//tick is a fresh full snapshot so the client just replaces its state; no diff
//merging.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>run_id</td>
<td>string</td>
<td><pre>
//run_id identifies the run to follow.<br>

json_name: runId
go_name: RunId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-submitregistrationrequestrequest"></a>
### cloud.v1.api.SubmitRegistrationRequestRequest

<pre>
//SubmitRegistrationRequestRequest is the PUBLIC submission. Accepted only when
//self-registration is disabled; otherwise the server tells the caller to
//Register instead. Idempotent on email: a repeat updates the existing row.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>email</td>
<td>string</td>
<td><pre>
json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>message</td>
<td>string</td>
<td><pre>
json_name: message
go_name: Message</pre></td>
</tr>
</table>



<a name="cloud-v1-api-submitregistrationrequestresponse"></a>
### cloud.v1.api.SubmitRegistrationRequestResponse

<pre>
//SubmitRegistrationRequestResponse is empty; success is signalled by the
//absence of error (no account-existence leak).
</pre>



<a name="cloud-v1-api-tenantdashboard"></a>
### cloud.v1.api.TenantDashboard

<pre>
//TenantDashboard is the whole landing payload.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>recent_runs</td>
<td><a href="../models/README.md#cloud-v1-models-run">cloud.v1.models.Run</a></td>
<td><pre>
//recent_runs is the most recent test runs (limited).<br>

json_name: recentRuns
go_name: RecentRuns</pre></td>
</tr><tr>
<td>run_counts</td>
<td><a href="#cloud-v1-api-statuscounts">cloud.v1.api.StatusCounts</a></td>
<td><pre>
//run_counts is the run breakdown by status (for the headline tiles).<br>

json_name: runCounts
go_name: RunCounts</pre></td>
</tr><tr>
<td>success_rate</td>
<td>float</td>
<td><pre>
//success_rate is completed / finished over a recent window, 0..1.<br>

json_name: successRate
go_name: SuccessRate</pre></td>
</tr><tr>
<td>top_benchmarks</td>
<td><a href="#cloud-v1-api-ratingentry">cloud.v1.api.RatingEntry</a></td>
<td><pre>
//top_benchmarks is this tenant's top benchmarks (tenant rating top-N).<br>

json_name: topBenchmarks
go_name: TopBenchmarks</pre></td>
</tr><tr>
<td>upcoming</td>
<td><a href="#cloud-v1-api-upcomingsuite">cloud.v1.api.UpcomingSuite</a></td>
<td><pre>
//upcoming is the scheduled suites with their next planned auto-run.<br>

json_name: upcoming
go_name: Upcoming</pre></td>
</tr>
</table>



<a name="cloud-v1-api-testrunoverviewsnapshot"></a>
### cloud.v1.api.TestRunOverviewSnapshot

<pre>
//TestRunOverviewSnapshot is the overview page state: persisted run record,
//current staged topology envelope, and live monitor pipeline/timeline view.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>overview</td>
<td><a href="../monitor/README.md#cloud-v1-monitor-overview">cloud.v1.monitor.Overview</a></td>
<td><pre>
//overview is the live status/pipeline/workers/timeline projection.<br>

json_name: overview
go_name: Overview</pre></td>
</tr><tr>
<td>run</td>
<td><a href="../models/README.md#cloud-v1-models-run">cloud.v1.models.Run</a></td>
<td><pre>
//run is the persisted run record, including immutable spec and any stored
//infrastructure/deployment artifacts.<br>

json_name: run
go_name: Run</pre></td>
</tr><tr>
<td>topology</td>
<td><a href="../topology/README.md#cloud-v1-topology-topology">cloud.v1.topology.Topology</a></td>
<td><pre>
//topology is the staged topology envelope composed from the run spec,
//infrastructure state and deployment plan.<br>

json_name: topology
go_name: Topology</pre></td>
</tr>
</table>



<a name="cloud-v1-api-tokenpair"></a>
### cloud.v1.api.TokenPair

<pre>
//TokenPair is the result of a successful Register, Login or Refresh. Both are
//bearer credentials.

//access_token is short-lived and sent on every API call (Authorization:
//Bearer). It is TENANT-AGNOSTIC (see iam/claims.proto) — one token works
//across every tenant the account can reach, so switching tenant never re-mints
//it.

//refresh_token is long-lived, single-use, and ROTATED on every Refresh: each
//Refresh invalidates the presented refresh_token and returns a new one, so the
//caller MUST persist the new refresh_token or the next Refresh fails. The
//server keeps the refresh token server-side so Logout can revoke it.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>access_expires_in</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//access_expires_in is the access_token lifetime from issuance.<br>

json_name: accessExpiresIn
go_name: AccessExpiresIn</pre></td>
</tr><tr>
<td>access_token</td>
<td>string</td>
<td><pre>
//access_token is the short-lived, tenant-agnostic bearer token.<br>

json_name: accessToken
go_name: AccessToken</pre></td>
</tr><tr>
<td>refresh_expires_in</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
//refresh_expires_in is the refresh_token lifetime from issuance.<br>

json_name: refreshExpiresIn
go_name: RefreshExpiresIn</pre></td>
</tr><tr>
<td>refresh_token</td>
<td>string</td>
<td><pre>
//refresh_token is the long-lived, single-use, rotated token.<br>

json_name: refreshToken
go_name: RefreshToken</pre></td>
</tr>
</table>



<a name="cloud-v1-api-transfertenantownershiprequest"></a>
### cloud.v1.api.TransferTenantOwnershipRequest

<pre>
//TransferTenantOwnershipRequest reassigns Tenant.owner_account_id to another
//account, which MUST already be a member of the tenant. Ownership is the only
//way to change owner_account_id (UpdateTenant cannot), keeping the transfer an
//explicit, auditable action.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>new_owner_account_id</td>
<td>string</td>
<td><pre>
//new_owner_account_id is the new owner; MUST already be a member.<br>

json_name: newOwnerAccountId
go_name: NewOwnerAccountId</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id is the tenant whose ownership is being reassigned.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-transfertenantownershipresponse"></a>
### cloud.v1.api.TransferTenantOwnershipResponse

<pre>
//TransferTenantOwnershipResponse returns the tenant after the transfer.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant</td>
<td><a href="../iam/README.md#cloud-v1-iam-tenant">cloud.v1.iam.Tenant</a></td>
<td><pre>
//tenant is the workspace with its new owner_account_id.<br>

json_name: tenant
go_name: Tenant</pre></td>
</tr>
</table>



<a name="cloud-v1-api-unlinkexternalidentityrequest"></a>
### cloud.v1.api.UnlinkExternalIdentityRequest

<pre>
//UnlinkExternalIdentityRequest removes one external identity by its id,
//revoking SSO login through it (the Account and other links remain). The
//caller must own the identity's account or be a platform admin (enforced
//server-side).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the external-identity link to remove.<br>

json_name: id
go_name: Id</pre></td>
</tr>
</table>



<a name="cloud-v1-api-unlinkexternalidentityresponse"></a>
### cloud.v1.api.UnlinkExternalIdentityResponse

<pre>
//UnlinkExternalIdentityResponse is empty; success is signalled by the absence
//of error.
</pre>



<a name="cloud-v1-api-upcomingsuite"></a>
### cloud.v1.api.UpcomingSuite

<pre>
//UpcomingSuite is a scheduled suite and its next planned auto-run.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>cron</td>
<td>string</td>
<td><pre>
//cron is the schedule expression driving the auto-runs.<br>

json_name: cron
go_name: Cron</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name is the suite's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>next_run_at</td>
<td><a href="../../../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
//next_run_at is when the next auto-run is planned.<br>

json_name: nextRunAt
go_name: NextRunAt</pre></td>
</tr><tr>
<td>suite_id</td>
<td>string</td>
<td><pre>
//suite_id identifies the scheduled suite definition.<br>

json_name: suiteId
go_name: SuiteId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updateaccountrequest"></a>
### cloud.v1.api.UpdateAccountRequest

<pre>
//UpdateAccountRequest mutates the named account. Only email/nickname are
//editable here; password rotation and is_admin changes are separate,
//privilege-gated operations.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>email</td>
<td>string</td>
<td><pre>
//email, when present, replaces the account's contact + login address.<br>

json_name: email
go_name: Email</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the account to mutate.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>nickname</td>
<td>string</td>
<td><pre>
//nickname, when present, replaces the account's handle.<br>

json_name: nickname
go_name: Nickname</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updateaccountresponse"></a>
### cloud.v1.api.UpdateAccountResponse

<pre>
//UpdateAccountResponse returns the account after the edit.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>account</td>
<td><a href="../iam/README.md#cloud-v1-iam-account">cloud.v1.iam.Account</a></td>
<td><pre>
//account is the updated global identity.<br>

json_name: account
go_name: Account</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updateidentityproviderrequest"></a>
### cloud.v1.api.UpdateIdentityProviderRequest

<pre>
//UpdateIdentityProviderRequest edits a provider. A present, non-empty
//client_secret ROTATES the stored secret; an absent one leaves it unchanged.
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
//allowed_domains, when present, replaces the email-domain allowlist.<br>

json_name: allowedDomains
go_name: AllowedDomains</pre></td>
</tr><tr>
<td>auto_provision</td>
<td>bool</td>
<td><pre>
//auto_provision, when present, toggles JIT account creation.<br>

json_name: autoProvision
go_name: AutoProvision</pre></td>
</tr><tr>
<td>client_id</td>
<td>string</td>
<td><pre>
//client_id, when present, replaces the OAuth client identifier.<br>

json_name: clientId
go_name: ClientId</pre></td>
</tr><tr>
<td>client_secret</td>
<td>string</td>
<td><pre>
//client_secret, when present and non-empty, ROTATES the stored secret;
//absent leaves it unchanged.<br>

json_name: clientSecret
go_name: ClientSecret</pre></td>
</tr><tr>
<td>disabled</td>
<td>bool</td>
<td><pre>
//disabled hides the provider (inverted polarity: false = enabled). See
//iam/sso.proto.<br>

json_name: disabled
go_name: Disabled</pre></td>
</tr><tr>
<td>display_name</td>
<td>string</td>
<td><pre>
//display_name, when present, replaces the login-button label.<br>

json_name: displayName
go_name: DisplayName</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the provider to edit.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>issuer</td>
<td>string</td>
<td><pre>
//issuer, when present, replaces the OIDC issuer URL.<br>

json_name: issuer
go_name: Issuer</pre></td>
</tr><tr>
<td>scopes</td>
<td>string</td>
<td><pre>
//scopes, when present, replaces the requested OIDC scopes.<br>

json_name: scopes
go_name: Scopes</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updateidentityproviderresponse"></a>
### cloud.v1.api.UpdateIdentityProviderResponse

<pre>
//UpdateIdentityProviderResponse returns the provider after the edit.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>provider</td>
<td><a href="../iam/README.md#cloud-v1-iam-identityprovider">cloud.v1.iam.IdentityProvider</a></td>
<td><pre>
//provider is the updated OIDC provider config (no secret).<br>

json_name: provider
go_name: Provider</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatemembershiprequest"></a>
### cloud.v1.api.UpdateMembershipRequest

<pre>
//UpdateMembershipRequest changes a member's granted roles. role_ids REPLACES
//the existing set wholesale; an empty set is rejected (remove the membership
//instead of leaving it role-less).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the membership to edit.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>role_ids</td>
<td>string</td>
<td><pre>
//role_ids REPLACES the granted role set wholesale; an empty set is rejected.<br>

json_name: roleIds
go_name: RoleIds</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatemembershipresponse"></a>
### cloud.v1.api.UpdateMembershipResponse

<pre>
//UpdateMembershipResponse returns the membership after the edit.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>membership</td>
<td><a href="../iam/README.md#cloud-v1-iam-membership">cloud.v1.iam.Membership</a></td>
<td><pre>
//membership is the updated grant.<br>

json_name: membership
go_name: Membership</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updaterolerequest"></a>
### cloud.v1.api.UpdateRoleRequest

<pre>
//UpdateRoleRequest edits a custom role. System roles (is_system) are rejected.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the custom role to edit (system roles are rejected).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name, when present, replaces the role's display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>permissions</td>
<td><a href="../iam/README.md#cloud-v1-iam-permission">cloud.v1.iam.Permission</a></td>
<td><pre>
//permissions, when present, REPLACES the role's permission set wholesale.<br>

json_name: permissions
go_name: Permissions</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updateroleresponse"></a>
### cloud.v1.api.UpdateRoleResponse

<pre>
//UpdateRoleResponse returns the role after the edit.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>role</td>
<td><a href="../iam/README.md#cloud-v1-iam-role">cloud.v1.iam.Role</a></td>
<td><pre>
//role is the updated role.<br>

json_name: role
go_name: Role</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatesystemsettingsrequest"></a>
### cloud.v1.api.UpdateSystemSettingsRequest

<pre>
//UpdateSystemSettings replaces the singleton wholesale. Idempotent.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="#cloud-v1-api-platformsettings">cloud.v1.api.PlatformSettings</a></td>
<td><pre>
//settings is the full replacement platform configuration.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatesystemsettingsresponse"></a>
### cloud.v1.api.UpdateSystemSettingsResponse

<pre>
//UpdateSystemSettingsResponse returns the settings after the update.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="#cloud-v1-api-platformsettings">cloud.v1.api.PlatformSettings</a></td>
<td><pre>
//settings is the updated singleton platform configuration.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatetenantrequest"></a>
### cloud.v1.api.UpdateTenantRequest

<pre>
//UpdateTenantRequest mutates name/slug. Renaming slug breaks old links.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td>string</td>
<td><pre>
//id is the tenant to mutate.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
//name, when present, replaces the display name.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>slug</td>
<td>string</td>
<td><pre>
//slug, when present, replaces the routing key (breaks old links).<br>

json_name: slug
go_name: Slug</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatetenantresponse"></a>
### cloud.v1.api.UpdateTenantResponse

<pre>
//UpdateTenantResponse returns the tenant after the edit.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>tenant</td>
<td><a href="../iam/README.md#cloud-v1-iam-tenant">cloud.v1.iam.Tenant</a></td>
<td><pre>
//tenant is the updated workspace.<br>

json_name: tenant
go_name: Tenant</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatetenantsettingsrequest"></a>
### cloud.v1.api.UpdateTenantSettingsRequest

<pre>
//UpdateTenantSettings replaces the tenant's settings wholesale. Idempotent.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="../models/README.md#cloud-v1-models-tenantsettingsrecord">cloud.v1.models.TenantSettingsRecord</a></td>
<td><pre>
//settings is the full replacement settings record.<br>

json_name: settings
go_name: Settings</pre></td>
</tr><tr>
<td>tenant_id</td>
<td>string</td>
<td><pre>
//tenant_id scopes the request to the owning tenant.<br>

json_name: tenantId
go_name: TenantId</pre></td>
</tr>
</table>



<a name="cloud-v1-api-updatetenantsettingsresponse"></a>
### cloud.v1.api.UpdateTenantSettingsResponse

<pre>
//UpdateTenantSettingsResponse returns the stored settings after the update.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>settings</td>
<td><a href="../models/README.md#cloud-v1-models-tenantsettingsrecord">cloud.v1.models.TenantSettingsRecord</a></td>
<td><pre>
//settings is the tenant's settings after the update.<br>

json_name: settings
go_name: Settings</pre></td>
</tr>
</table>



<a name="cloud-v1-api-verifyemailrequest"></a>
### cloud.v1.api.VerifyEmailRequest

<pre>
//VerifyEmailRequest consumes an emailed verification token and flips the
//target account's Account.email_verified to true. PUBLIC: the token is the
//credential and the user may not be logged in when clicking the link.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>token</td>
<td>string</td>
<td><pre>
//token is the single-use emailed verification token (the credential).<br>

json_name: token
go_name: Token</pre></td>
</tr>
</table>



<a name="cloud-v1-api-verifyemailresponse"></a>
### cloud.v1.api.VerifyEmailResponse

<pre>
//VerifyEmailResponse is empty; success is signalled by the absence of error.
</pre>

