# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [cloud/v1/agent/logs.proto](#cloud_v1_agent_logs-proto)
    - [LogBatch](#cloud-v1-agent-LogBatch)
    - [ShipLogsAck](#cloud-v1-agent-ShipLogsAck)
  
    - [AgentLogService](#cloud-v1-agent-AgentLogService)
  
- [cloud/v1/agent/registry.proto](#cloud_v1_agent_registry-proto)
    - [AgentInfo](#cloud-v1-agent-AgentInfo)
    - [HeartbeatRequest](#cloud-v1-agent-HeartbeatRequest)
    - [HeartbeatResponse](#cloud-v1-agent-HeartbeatResponse)
    - [RegisterRequest](#cloud-v1-agent-RegisterRequest)
    - [RegisterResponse](#cloud-v1-agent-RegisterResponse)
  
    - [AgentRegistryService](#cloud-v1-agent-AgentRegistryService)
  
- [cloud/v1/agent/shell.proto](#cloud_v1_agent_shell-proto)
    - [AgentShellMsg](#cloud-v1-agent-AgentShellMsg)
    - [OpenShell](#cloud-v1-agent-OpenShell)
    - [Register](#cloud-v1-agent-Register)
    - [ServerShellMsg](#cloud-v1-agent-ServerShellMsg)
    - [ShellExit](#cloud-v1-agent-ShellExit)
    - [ShellResize](#cloud-v1-agent-ShellResize)
  
    - [AgentShellAgentService](#cloud-v1-agent-AgentShellAgentService)
  
- [cloud/v1/api/admin.proto](#cloud_v1_api_admin-proto)
    - [PlatformSettings](#cloud-v1-api-PlatformSettings)
  
- [cloud/v1/api/agent_shell.proto](#cloud_v1_api_agent_shell-proto)
    - [ShellClientFrame](#cloud-v1-api-ShellClientFrame)
    - [ShellServerFrame](#cloud-v1-api-ShellServerFrame)
    - [ShellStart](#cloud-v1-api-ShellStart)
  
    - [AgentShellService](#cloud-v1-api-AgentShellService)
  
- [cloud/v1/api/compare.proto](#cloud_v1_api_compare-proto)
    - [CompareRunsRequest](#cloud-v1-api-CompareRunsRequest)
    - [CompareRunsResponse](#cloud-v1-api-CompareRunsResponse)
    - [CompareView](#cloud-v1-api-CompareView)
    - [RunColumn](#cloud-v1-api-RunColumn)
  
    - [CompareService](#cloud-v1-api-CompareService)
  
- [cloud/v1/api/favorite.proto](#cloud_v1_api_favorite-proto)
    - [AddFavoriteRequest](#cloud-v1-api-AddFavoriteRequest)
    - [AddFavoriteResponse](#cloud-v1-api-AddFavoriteResponse)
    - [ListFavoritesRequest](#cloud-v1-api-ListFavoritesRequest)
    - [ListFavoritesResponse](#cloud-v1-api-ListFavoritesResponse)
    - [RemoveFavoriteRequest](#cloud-v1-api-RemoveFavoriteRequest)
    - [RemoveFavoriteResponse](#cloud-v1-api-RemoveFavoriteResponse)
  
    - [FavoriteService](#cloud-v1-api-FavoriteService)
  
- [cloud/v1/api/iam.proto](#cloud_v1_api_iam-proto)
    - [CatalogEntry](#cloud-v1-api-CatalogEntry)
    - [ChangePasswordRequest](#cloud-v1-api-ChangePasswordRequest)
    - [ChangePasswordResponse](#cloud-v1-api-ChangePasswordResponse)
    - [CompleteSSORequest](#cloud-v1-api-CompleteSSORequest)
    - [CompleteSSOResponse](#cloud-v1-api-CompleteSSOResponse)
    - [ConfirmPasswordResetRequest](#cloud-v1-api-ConfirmPasswordResetRequest)
    - [ConfirmPasswordResetResponse](#cloud-v1-api-ConfirmPasswordResetResponse)
    - [CreateAccountRequest](#cloud-v1-api-CreateAccountRequest)
    - [CreateAccountResponse](#cloud-v1-api-CreateAccountResponse)
    - [CreateApiTokenRequest](#cloud-v1-api-CreateApiTokenRequest)
    - [CreateApiTokenResponse](#cloud-v1-api-CreateApiTokenResponse)
    - [CreateIdentityProviderRequest](#cloud-v1-api-CreateIdentityProviderRequest)
    - [CreateIdentityProviderResponse](#cloud-v1-api-CreateIdentityProviderResponse)
    - [CreateMembershipRequest](#cloud-v1-api-CreateMembershipRequest)
    - [CreateMembershipResponse](#cloud-v1-api-CreateMembershipResponse)
    - [CreateRoleRequest](#cloud-v1-api-CreateRoleRequest)
    - [CreateRoleResponse](#cloud-v1-api-CreateRoleResponse)
    - [CreateTenantRequest](#cloud-v1-api-CreateTenantRequest)
    - [CreateTenantResponse](#cloud-v1-api-CreateTenantResponse)
    - [DeleteAccountRequest](#cloud-v1-api-DeleteAccountRequest)
    - [DeleteAccountResponse](#cloud-v1-api-DeleteAccountResponse)
    - [DeleteIdentityProviderRequest](#cloud-v1-api-DeleteIdentityProviderRequest)
    - [DeleteIdentityProviderResponse](#cloud-v1-api-DeleteIdentityProviderResponse)
    - [DeleteMembershipRequest](#cloud-v1-api-DeleteMembershipRequest)
    - [DeleteMembershipResponse](#cloud-v1-api-DeleteMembershipResponse)
    - [DeleteRoleRequest](#cloud-v1-api-DeleteRoleRequest)
    - [DeleteRoleResponse](#cloud-v1-api-DeleteRoleResponse)
    - [DeleteTenantRequest](#cloud-v1-api-DeleteTenantRequest)
    - [DeleteTenantResponse](#cloud-v1-api-DeleteTenantResponse)
    - [ExternalIdentityLink](#cloud-v1-api-ExternalIdentityLink)
    - [GetAccountRequest](#cloud-v1-api-GetAccountRequest)
    - [GetAccountResponse](#cloud-v1-api-GetAccountResponse)
    - [GetIdentityProviderRequest](#cloud-v1-api-GetIdentityProviderRequest)
    - [GetIdentityProviderResponse](#cloud-v1-api-GetIdentityProviderResponse)
    - [GetMembershipRequest](#cloud-v1-api-GetMembershipRequest)
    - [GetMembershipResponse](#cloud-v1-api-GetMembershipResponse)
    - [GetMyAccountRequest](#cloud-v1-api-GetMyAccountRequest)
    - [GetMyAccountResponse](#cloud-v1-api-GetMyAccountResponse)
    - [GetMyPermissionsRequest](#cloud-v1-api-GetMyPermissionsRequest)
    - [GetMyPermissionsResponse](#cloud-v1-api-GetMyPermissionsResponse)
    - [GetRoleRequest](#cloud-v1-api-GetRoleRequest)
    - [GetRoleResponse](#cloud-v1-api-GetRoleResponse)
    - [GetTenantRequest](#cloud-v1-api-GetTenantRequest)
    - [GetTenantResponse](#cloud-v1-api-GetTenantResponse)
    - [LeaveTenantRequest](#cloud-v1-api-LeaveTenantRequest)
    - [LeaveTenantResponse](#cloud-v1-api-LeaveTenantResponse)
    - [LinkExternalIdentityRequest](#cloud-v1-api-LinkExternalIdentityRequest)
    - [LinkExternalIdentityResponse](#cloud-v1-api-LinkExternalIdentityResponse)
    - [ListAccountsRequest](#cloud-v1-api-ListAccountsRequest)
    - [ListAccountsResponse](#cloud-v1-api-ListAccountsResponse)
    - [ListApiTokensRequest](#cloud-v1-api-ListApiTokensRequest)
    - [ListApiTokensResponse](#cloud-v1-api-ListApiTokensResponse)
    - [ListExternalIdentitiesRequest](#cloud-v1-api-ListExternalIdentitiesRequest)
    - [ListExternalIdentitiesResponse](#cloud-v1-api-ListExternalIdentitiesResponse)
    - [ListIdentityProvidersRequest](#cloud-v1-api-ListIdentityProvidersRequest)
    - [ListIdentityProvidersResponse](#cloud-v1-api-ListIdentityProvidersResponse)
    - [ListMembershipsRequest](#cloud-v1-api-ListMembershipsRequest)
    - [ListMembershipsResponse](#cloud-v1-api-ListMembershipsResponse)
    - [ListMyTenantsRequest](#cloud-v1-api-ListMyTenantsRequest)
    - [ListMyTenantsResponse](#cloud-v1-api-ListMyTenantsResponse)
    - [ListPermissionsRequest](#cloud-v1-api-ListPermissionsRequest)
    - [ListPermissionsResponse](#cloud-v1-api-ListPermissionsResponse)
    - [ListRolesRequest](#cloud-v1-api-ListRolesRequest)
    - [ListRolesResponse](#cloud-v1-api-ListRolesResponse)
    - [LoginRequest](#cloud-v1-api-LoginRequest)
    - [LoginResponse](#cloud-v1-api-LoginResponse)
    - [LogoutRequest](#cloud-v1-api-LogoutRequest)
    - [LogoutResponse](#cloud-v1-api-LogoutResponse)
    - [RefreshRequest](#cloud-v1-api-RefreshRequest)
    - [RefreshResponse](#cloud-v1-api-RefreshResponse)
    - [RegisterRequest](#cloud-v1-api-RegisterRequest)
    - [RegisterResponse](#cloud-v1-api-RegisterResponse)
    - [RequestPasswordResetRequest](#cloud-v1-api-RequestPasswordResetRequest)
    - [RequestPasswordResetResponse](#cloud-v1-api-RequestPasswordResetResponse)
    - [ResendVerificationRequest](#cloud-v1-api-ResendVerificationRequest)
    - [ResendVerificationResponse](#cloud-v1-api-ResendVerificationResponse)
    - [ResetPasswordRequest](#cloud-v1-api-ResetPasswordRequest)
    - [ResetPasswordResponse](#cloud-v1-api-ResetPasswordResponse)
    - [RevokeApiTokenRequest](#cloud-v1-api-RevokeApiTokenRequest)
    - [RevokeApiTokenResponse](#cloud-v1-api-RevokeApiTokenResponse)
    - [SsoButton](#cloud-v1-api-SsoButton)
    - [StartSSORequest](#cloud-v1-api-StartSSORequest)
    - [StartSSOResponse](#cloud-v1-api-StartSSOResponse)
    - [TokenPair](#cloud-v1-api-TokenPair)
    - [TransferTenantOwnershipRequest](#cloud-v1-api-TransferTenantOwnershipRequest)
    - [TransferTenantOwnershipResponse](#cloud-v1-api-TransferTenantOwnershipResponse)
    - [UnlinkExternalIdentityRequest](#cloud-v1-api-UnlinkExternalIdentityRequest)
    - [UnlinkExternalIdentityResponse](#cloud-v1-api-UnlinkExternalIdentityResponse)
    - [UpdateAccountRequest](#cloud-v1-api-UpdateAccountRequest)
    - [UpdateAccountResponse](#cloud-v1-api-UpdateAccountResponse)
    - [UpdateIdentityProviderRequest](#cloud-v1-api-UpdateIdentityProviderRequest)
    - [UpdateIdentityProviderResponse](#cloud-v1-api-UpdateIdentityProviderResponse)
    - [UpdateMembershipRequest](#cloud-v1-api-UpdateMembershipRequest)
    - [UpdateMembershipResponse](#cloud-v1-api-UpdateMembershipResponse)
    - [UpdateRoleRequest](#cloud-v1-api-UpdateRoleRequest)
    - [UpdateRoleResponse](#cloud-v1-api-UpdateRoleResponse)
    - [UpdateTenantRequest](#cloud-v1-api-UpdateTenantRequest)
    - [UpdateTenantResponse](#cloud-v1-api-UpdateTenantResponse)
    - [VerifyEmailRequest](#cloud-v1-api-VerifyEmailRequest)
    - [VerifyEmailResponse](#cloud-v1-api-VerifyEmailResponse)
  
    - [IamService](#cloud-v1-api-IamService)
  
- [cloud/v1/api/package.proto](#cloud_v1_api_package-proto)
    - [CompleteUploadRequest](#cloud-v1-api-CompleteUploadRequest)
    - [CompleteUploadResponse](#cloud-v1-api-CompleteUploadResponse)
    - [CreatePackageUploadRequest](#cloud-v1-api-CreatePackageUploadRequest)
    - [CreatePackageUploadResponse](#cloud-v1-api-CreatePackageUploadResponse)
    - [DeletePackageRequest](#cloud-v1-api-DeletePackageRequest)
    - [DeletePackageResponse](#cloud-v1-api-DeletePackageResponse)
    - [GetPackageRequest](#cloud-v1-api-GetPackageRequest)
    - [GetPackageResponse](#cloud-v1-api-GetPackageResponse)
    - [ListPackagesRequest](#cloud-v1-api-ListPackagesRequest)
    - [ListPackagesResponse](#cloud-v1-api-ListPackagesResponse)
  
    - [PackageService](#cloud-v1-api-PackageService)
  
- [cloud/v1/api/preset.proto](#cloud_v1_api_preset-proto)
    - [CloneDatabasePresetRequest](#cloud-v1-api-CloneDatabasePresetRequest)
    - [CloneDatabasePresetResponse](#cloud-v1-api-CloneDatabasePresetResponse)
    - [CloneTestPresetRequest](#cloud-v1-api-CloneTestPresetRequest)
    - [CloneTestPresetResponse](#cloud-v1-api-CloneTestPresetResponse)
    - [CloneWorkloadPresetRequest](#cloud-v1-api-CloneWorkloadPresetRequest)
    - [CloneWorkloadPresetResponse](#cloud-v1-api-CloneWorkloadPresetResponse)
    - [CreateDatabasePresetRequest](#cloud-v1-api-CreateDatabasePresetRequest)
    - [CreateDatabasePresetResponse](#cloud-v1-api-CreateDatabasePresetResponse)
    - [CreateTestPresetRequest](#cloud-v1-api-CreateTestPresetRequest)
    - [CreateTestPresetResponse](#cloud-v1-api-CreateTestPresetResponse)
    - [CreateWorkloadPresetRequest](#cloud-v1-api-CreateWorkloadPresetRequest)
    - [CreateWorkloadPresetResponse](#cloud-v1-api-CreateWorkloadPresetResponse)
    - [DeleteDatabasePresetRequest](#cloud-v1-api-DeleteDatabasePresetRequest)
    - [DeleteDatabasePresetResponse](#cloud-v1-api-DeleteDatabasePresetResponse)
    - [DeleteTestPresetRequest](#cloud-v1-api-DeleteTestPresetRequest)
    - [DeleteTestPresetResponse](#cloud-v1-api-DeleteTestPresetResponse)
    - [DeleteWorkloadPresetRequest](#cloud-v1-api-DeleteWorkloadPresetRequest)
    - [DeleteWorkloadPresetResponse](#cloud-v1-api-DeleteWorkloadPresetResponse)
    - [GetDatabasePresetRequest](#cloud-v1-api-GetDatabasePresetRequest)
    - [GetDatabasePresetResponse](#cloud-v1-api-GetDatabasePresetResponse)
    - [GetTestPresetRequest](#cloud-v1-api-GetTestPresetRequest)
    - [GetTestPresetResponse](#cloud-v1-api-GetTestPresetResponse)
    - [GetWorkloadPresetRequest](#cloud-v1-api-GetWorkloadPresetRequest)
    - [GetWorkloadPresetResponse](#cloud-v1-api-GetWorkloadPresetResponse)
    - [ListDatabasePresetsRequest](#cloud-v1-api-ListDatabasePresetsRequest)
    - [ListDatabasePresetsRequest.Sort](#cloud-v1-api-ListDatabasePresetsRequest-Sort)
    - [ListDatabasePresetsRequest.TagsEntry](#cloud-v1-api-ListDatabasePresetsRequest-TagsEntry)
    - [ListDatabasePresetsResponse](#cloud-v1-api-ListDatabasePresetsResponse)
    - [ListTestPresetsRequest](#cloud-v1-api-ListTestPresetsRequest)
    - [ListTestPresetsRequest.Sort](#cloud-v1-api-ListTestPresetsRequest-Sort)
    - [ListTestPresetsRequest.TagsEntry](#cloud-v1-api-ListTestPresetsRequest-TagsEntry)
    - [ListTestPresetsResponse](#cloud-v1-api-ListTestPresetsResponse)
    - [ListWorkloadPresetsRequest](#cloud-v1-api-ListWorkloadPresetsRequest)
    - [ListWorkloadPresetsRequest.Sort](#cloud-v1-api-ListWorkloadPresetsRequest-Sort)
    - [ListWorkloadPresetsRequest.TagsEntry](#cloud-v1-api-ListWorkloadPresetsRequest-TagsEntry)
    - [ListWorkloadPresetsResponse](#cloud-v1-api-ListWorkloadPresetsResponse)
    - [UpdateDatabasePresetRequest](#cloud-v1-api-UpdateDatabasePresetRequest)
    - [UpdateDatabasePresetResponse](#cloud-v1-api-UpdateDatabasePresetResponse)
    - [UpdateTestPresetRequest](#cloud-v1-api-UpdateTestPresetRequest)
    - [UpdateTestPresetResponse](#cloud-v1-api-UpdateTestPresetResponse)
    - [UpdateWorkloadPresetRequest](#cloud-v1-api-UpdateWorkloadPresetRequest)
    - [UpdateWorkloadPresetResponse](#cloud-v1-api-UpdateWorkloadPresetResponse)
  
    - [ListDatabasePresetsRequest.Sort.Kind](#cloud-v1-api-ListDatabasePresetsRequest-Sort-Kind)
    - [ListDatabasePresetsRequest.SourceKind](#cloud-v1-api-ListDatabasePresetsRequest-SourceKind)
    - [ListTestPresetsRequest.Sort.Kind](#cloud-v1-api-ListTestPresetsRequest-Sort-Kind)
    - [ListWorkloadPresetsRequest.Sort.Kind](#cloud-v1-api-ListWorkloadPresetsRequest-Sort-Kind)
  
    - [DatabasePresetService](#cloud-v1-api-DatabasePresetService)
    - [TestPresetService](#cloud-v1-api-TestPresetService)
    - [WorkloadPresetService](#cloud-v1-api-WorkloadPresetService)
  
- [cloud/v1/api/public_rating.proto](#cloud_v1_api_public_rating-proto)
    - [GetPublicRatingRequest](#cloud-v1-api-GetPublicRatingRequest)
    - [GetPublicRatingResponse](#cloud-v1-api-GetPublicRatingResponse)
    - [PublicRatingEntry](#cloud-v1-api-PublicRatingEntry)
  
    - [PublicRatingService](#cloud-v1-api-PublicRatingService)
  
- [cloud/v1/api/public_share.proto](#cloud_v1_api_public_share-proto)
    - [GetSharedRunRequest](#cloud-v1-api-GetSharedRunRequest)
    - [GetSharedRunResponse](#cloud-v1-api-GetSharedRunResponse)
  
    - [PublicShareService](#cloud-v1-api-PublicShareService)
  
- [cloud/v1/api/rating.proto](#cloud_v1_api_rating-proto)
    - [GetSystemRatingRequest](#cloud-v1-api-GetSystemRatingRequest)
    - [GetSystemRatingResponse](#cloud-v1-api-GetSystemRatingResponse)
    - [GetTenantRatingRequest](#cloud-v1-api-GetTenantRatingRequest)
    - [GetTenantRatingResponse](#cloud-v1-api-GetTenantRatingResponse)
    - [RatingEntry](#cloud-v1-api-RatingEntry)
    - [RatingFilter](#cloud-v1-api-RatingFilter)
  
    - [RatingService](#cloud-v1-api-RatingService)
  
- [cloud/v1/api/share.proto](#cloud_v1_api_share-proto)
    - [CreateShareRequest](#cloud-v1-api-CreateShareRequest)
    - [CreateShareResponse](#cloud-v1-api-CreateShareResponse)
    - [DeleteShareRequest](#cloud-v1-api-DeleteShareRequest)
    - [DeleteShareResponse](#cloud-v1-api-DeleteShareResponse)
    - [GetShareRequest](#cloud-v1-api-GetShareRequest)
    - [GetShareResponse](#cloud-v1-api-GetShareResponse)
    - [ListSharesRequest](#cloud-v1-api-ListSharesRequest)
    - [ListSharesResponse](#cloud-v1-api-ListSharesResponse)
    - [RevokeShareRequest](#cloud-v1-api-RevokeShareRequest)
    - [RevokeShareResponse](#cloud-v1-api-RevokeShareResponse)
    - [SetShareExpiryRequest](#cloud-v1-api-SetShareExpiryRequest)
    - [SetShareExpiryResponse](#cloud-v1-api-SetShareExpiryResponse)
  
    - [ShareService](#cloud-v1-api-ShareService)
  
- [cloud/v1/api/suite.proto](#cloud_v1_api_suite-proto)
    - [CloneSuiteRequest](#cloud-v1-api-CloneSuiteRequest)
    - [CloneSuiteResponse](#cloud-v1-api-CloneSuiteResponse)
    - [CreateSuiteRequest](#cloud-v1-api-CreateSuiteRequest)
    - [CreateSuiteResponse](#cloud-v1-api-CreateSuiteResponse)
    - [DeleteSuiteRequest](#cloud-v1-api-DeleteSuiteRequest)
    - [DeleteSuiteResponse](#cloud-v1-api-DeleteSuiteResponse)
    - [GetSuiteRequest](#cloud-v1-api-GetSuiteRequest)
    - [GetSuiteResponse](#cloud-v1-api-GetSuiteResponse)
    - [ListSuitesRequest](#cloud-v1-api-ListSuitesRequest)
    - [ListSuitesRequest.Sort](#cloud-v1-api-ListSuitesRequest-Sort)
    - [ListSuitesResponse](#cloud-v1-api-ListSuitesResponse)
    - [SetSuiteScheduleRequest](#cloud-v1-api-SetSuiteScheduleRequest)
    - [SetSuiteScheduleResponse](#cloud-v1-api-SetSuiteScheduleResponse)
    - [StartSuiteRequest](#cloud-v1-api-StartSuiteRequest)
    - [StartSuiteResponse](#cloud-v1-api-StartSuiteResponse)
    - [UpdateSuiteRequest](#cloud-v1-api-UpdateSuiteRequest)
    - [UpdateSuiteResponse](#cloud-v1-api-UpdateSuiteResponse)
  
    - [ListSuitesRequest.Sort.Kind](#cloud-v1-api-ListSuitesRequest-Sort-Kind)
  
    - [SuiteService](#cloud-v1-api-SuiteService)
  
- [cloud/v1/api/suite_run.proto](#cloud_v1_api_suite_run-proto)
    - [CancelSuiteRunRequest](#cloud-v1-api-CancelSuiteRunRequest)
    - [CancelSuiteRunResponse](#cloud-v1-api-CancelSuiteRunResponse)
    - [DeleteSuiteRunRequest](#cloud-v1-api-DeleteSuiteRunRequest)
    - [DeleteSuiteRunResponse](#cloud-v1-api-DeleteSuiteRunResponse)
    - [GetSuiteRunRequest](#cloud-v1-api-GetSuiteRunRequest)
    - [GetSuiteRunResponse](#cloud-v1-api-GetSuiteRunResponse)
    - [ListSuiteRunsRequest](#cloud-v1-api-ListSuiteRunsRequest)
    - [ListSuiteRunsRequest.Sort](#cloud-v1-api-ListSuiteRunsRequest-Sort)
    - [ListSuiteRunsResponse](#cloud-v1-api-ListSuiteRunsResponse)
  
    - [ListSuiteRunsRequest.Sort.Kind](#cloud-v1-api-ListSuiteRunsRequest-Sort-Kind)
  
    - [SuiteRunService](#cloud-v1-api-SuiteRunService)
  
- [cloud/v1/api/suite_wizard.proto](#cloud_v1_api_suite_wizard-proto)
    - [DeleteSuiteWizardDraftRequest](#cloud-v1-api-DeleteSuiteWizardDraftRequest)
    - [DeleteSuiteWizardDraftResponse](#cloud-v1-api-DeleteSuiteWizardDraftResponse)
    - [FinishSuiteWizardRequest](#cloud-v1-api-FinishSuiteWizardRequest)
    - [FinishSuiteWizardResponse](#cloud-v1-api-FinishSuiteWizardResponse)
    - [GetSuiteWizardDraftRequest](#cloud-v1-api-GetSuiteWizardDraftRequest)
    - [GetSuiteWizardDraftResponse](#cloud-v1-api-GetSuiteWizardDraftResponse)
    - [ListSuiteWizardDraftsRequest](#cloud-v1-api-ListSuiteWizardDraftsRequest)
    - [ListSuiteWizardDraftsResponse](#cloud-v1-api-ListSuiteWizardDraftsResponse)
    - [PatchSuiteWizardRequest](#cloud-v1-api-PatchSuiteWizardRequest)
    - [PatchSuiteWizardResponse](#cloud-v1-api-PatchSuiteWizardResponse)
    - [StartSuiteWizardRequest](#cloud-v1-api-StartSuiteWizardRequest)
    - [StartSuiteWizardResponse](#cloud-v1-api-StartSuiteWizardResponse)
  
    - [SuiteWizardService](#cloud-v1-api-SuiteWizardService)
  
- [cloud/v1/api/system_settings.proto](#cloud_v1_api_system_settings-proto)
    - [GetSystemSettingsRequest](#cloud-v1-api-GetSystemSettingsRequest)
    - [GetSystemSettingsResponse](#cloud-v1-api-GetSystemSettingsResponse)
    - [UpdateSystemSettingsRequest](#cloud-v1-api-UpdateSystemSettingsRequest)
    - [UpdateSystemSettingsResponse](#cloud-v1-api-UpdateSystemSettingsResponse)
  
    - [SystemSettingsService](#cloud-v1-api-SystemSettingsService)
  
- [cloud/v1/api/tenant_dashboard.proto](#cloud_v1_api_tenant_dashboard-proto)
    - [GetTenantDashboardRequest](#cloud-v1-api-GetTenantDashboardRequest)
    - [GetTenantDashboardResponse](#cloud-v1-api-GetTenantDashboardResponse)
    - [StatusCounts](#cloud-v1-api-StatusCounts)
    - [TenantDashboard](#cloud-v1-api-TenantDashboard)
    - [UpcomingSuite](#cloud-v1-api-UpcomingSuite)
  
    - [TenantDashboardService](#cloud-v1-api-TenantDashboardService)
  
- [cloud/v1/api/tenant_settings.proto](#cloud_v1_api_tenant_settings-proto)
    - [GetTenantSettingsRequest](#cloud-v1-api-GetTenantSettingsRequest)
    - [GetTenantSettingsResponse](#cloud-v1-api-GetTenantSettingsResponse)
    - [SetTenantProviderSettingsRequest](#cloud-v1-api-SetTenantProviderSettingsRequest)
    - [SetTenantProviderSettingsResponse](#cloud-v1-api-SetTenantProviderSettingsResponse)
    - [UpdateTenantSettingsRequest](#cloud-v1-api-UpdateTenantSettingsRequest)
    - [UpdateTenantSettingsResponse](#cloud-v1-api-UpdateTenantSettingsResponse)
  
    - [TenantSettingsService](#cloud-v1-api-TenantSettingsService)
  
- [cloud/v1/api/test.proto](#cloud_v1_api_test-proto)
- [cloud/v1/api/test_run.proto](#cloud_v1_api_test_run-proto)
    - [CancelTestRunRequest](#cloud-v1-api-CancelTestRunRequest)
    - [CancelTestRunResponse](#cloud-v1-api-CancelTestRunResponse)
    - [DeleteTestRunRequest](#cloud-v1-api-DeleteTestRunRequest)
    - [DeleteTestRunResponse](#cloud-v1-api-DeleteTestRunResponse)
    - [ExtractToPresetRequest](#cloud-v1-api-ExtractToPresetRequest)
    - [ExtractToPresetResponse](#cloud-v1-api-ExtractToPresetResponse)
    - [GetTestRunRequest](#cloud-v1-api-GetTestRunRequest)
    - [GetTestRunResponse](#cloud-v1-api-GetTestRunResponse)
    - [ListTestRunsRequest](#cloud-v1-api-ListTestRunsRequest)
    - [ListTestRunsRequest.Sort](#cloud-v1-api-ListTestRunsRequest-Sort)
    - [ListTestRunsResponse](#cloud-v1-api-ListTestRunsResponse)
    - [StartTestRunRequest](#cloud-v1-api-StartTestRunRequest)
    - [StartTestRunResponse](#cloud-v1-api-StartTestRunResponse)
  
    - [ListTestRunsRequest.Sort.Kind](#cloud-v1-api-ListTestRunsRequest-Sort-Kind)
  
    - [TestRunService](#cloud-v1-api-TestRunService)
  
- [cloud/v1/api/test_run_overview.proto](#cloud_v1_api_test_run_overview-proto)
    - [GetRunMetricsRequest](#cloud-v1-api-GetRunMetricsRequest)
    - [GetRunMetricsResponse](#cloud-v1-api-GetRunMetricsResponse)
    - [GetTestRunOverviewRequest](#cloud-v1-api-GetTestRunOverviewRequest)
    - [GetTestRunOverviewResponse](#cloud-v1-api-GetTestRunOverviewResponse)
    - [LogFilter](#cloud-v1-api-LogFilter)
    - [QueryLogsRequest](#cloud-v1-api-QueryLogsRequest)
    - [QueryLogsResponse](#cloud-v1-api-QueryLogsResponse)
    - [ResolveLogRefRequest](#cloud-v1-api-ResolveLogRefRequest)
    - [ResolveLogRefResponse](#cloud-v1-api-ResolveLogRefResponse)
    - [StreamLogsRequest](#cloud-v1-api-StreamLogsRequest)
    - [StreamTestRunOverviewRequest](#cloud-v1-api-StreamTestRunOverviewRequest)
  
    - [LogScrollDirection](#cloud-v1-api-LogScrollDirection)
  
    - [TestRunOverviewService](#cloud-v1-api-TestRunOverviewService)
  
- [cloud/v1/api/test_wizard.proto](#cloud_v1_api_test_wizard-proto)
    - [DeleteTestWizardDraftRequest](#cloud-v1-api-DeleteTestWizardDraftRequest)
    - [DeleteTestWizardDraftResponse](#cloud-v1-api-DeleteTestWizardDraftResponse)
    - [FinishTestWizardRequest](#cloud-v1-api-FinishTestWizardRequest)
    - [FinishTestWizardResponse](#cloud-v1-api-FinishTestWizardResponse)
    - [GetTestWizardDraftRequest](#cloud-v1-api-GetTestWizardDraftRequest)
    - [GetTestWizardDraftResponse](#cloud-v1-api-GetTestWizardDraftResponse)
    - [ListTestWizardDraftsRequest](#cloud-v1-api-ListTestWizardDraftsRequest)
    - [ListTestWizardDraftsResponse](#cloud-v1-api-ListTestWizardDraftsResponse)
    - [PatchTestWizardRequest](#cloud-v1-api-PatchTestWizardRequest)
    - [PatchTestWizardResponse](#cloud-v1-api-PatchTestWizardResponse)
    - [StartTestWizardRequest](#cloud-v1-api-StartTestWizardRequest)
    - [StartTestWizardResponse](#cloud-v1-api-StartTestWizardResponse)
  
    - [TestWizardService](#cloud-v1-api-TestWizardService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="cloud_v1_agent_logs-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/agent/logs.proto



<a name="cloud-v1-agent-LogBatch"></a>

### LogBatch
LogBatch is a chunk of lines the agent flushes together (by count or time).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lines | [cloud.v1.monitor.LogLine](#cloud-v1-monitor-LogLine) | repeated | lines is the batch of log lines to ship; capped at 10000 per batch so a single message stays bounded. |






<a name="cloud-v1-agent-ShipLogsAck"></a>

### ShipLogsAck
ShipLogsAck is the server&#39;s running acknowledgement back to the agent on the
log stream.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| accepted | [uint64](#uint64) |  | accepted is how many lines the server accepted across this stream so far (used for backpressure). |





 

 

 


<a name="cloud-v1-agent-AgentLogService"></a>

### AgentLogService
AgentLogService is the agent-facing log ingestion endpoint that forwards
shipped lines into VictoriaLogs.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ShipLogs | [LogBatch](#cloud-v1-agent-LogBatch) stream | [ShipLogsAck](#cloud-v1-agent-ShipLogsAck) | ShipLogs is a client stream of batches: the agent flushes LogBatch chunks continuously; the server writes them to VictoriaLogs and acks counts. |

 



<a name="cloud_v1_agent_registry-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/agent/registry.proto



<a name="cloud-v1-agent-AgentInfo"></a>

### AgentInfo
AgentInfo identifies an agent host.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| machine_id | [string](#string) |  | machine_id is the stable per-host identifier the agent registers under. |
| host | [string](#string) |  | host is the agent&#39;s hostname / network address (informational). |
| agent_version | [string](#string) |  | agent_version is the running agent build version (for compatibility/audit). |
| run_id | [string](#string) |  | run_id is the run this agent is provisioned for (scoping/audit), if any. |






<a name="cloud-v1-agent-HeartbeatRequest"></a>

### HeartbeatRequest
HeartbeatRequest keeps an already-registered agent marked online.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| machine_id | [string](#string) |  | machine_id is the host whose liveness is being refreshed. |






<a name="cloud-v1-agent-HeartbeatResponse"></a>

### HeartbeatResponse
HeartbeatResponse is the empty server acknowledgement of a heartbeat.






<a name="cloud-v1-agent-RegisterRequest"></a>

### RegisterRequest
RegisterRequest announces an agent coming online to the server.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| info | [AgentInfo](#cloud-v1-agent-AgentInfo) |  | info is the identifying details of the agent host coming online. |






<a name="cloud-v1-agent-RegisterResponse"></a>

### RegisterResponse
RegisterResponse tells the agent it is registered and how often to heartbeat.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| registered_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | registered_at is the server clock time the agent was recorded online. |
| heartbeat_interval_seconds | [uint32](#uint32) |  | heartbeat_interval_seconds is the heartbeat cadence the server expects; the agent is considered offline after a missed window. |





 

 

 


<a name="cloud-v1-agent-AgentRegistryService"></a>

### AgentRegistryService
AgentRegistryService is the agent-facing presence channel: agents register
and heartbeat so the server knows which agents are online.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Register | [RegisterRequest](#cloud-v1-agent-RegisterRequest) | [RegisterResponse](#cloud-v1-agent-RegisterResponse) | Register announces an agent coming online. |
| Heartbeat | [HeartbeatRequest](#cloud-v1-agent-HeartbeatRequest) | [HeartbeatResponse](#cloud-v1-agent-HeartbeatResponse) | Heartbeat keeps the agent marked online. |

 



<a name="cloud_v1_agent_shell-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/agent/shell.proto



<a name="cloud-v1-agent-AgentShellMsg"></a>

### AgentShellMsg
AgentShellMsg is agent -&gt; server over the control stream.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id is the session this frame belongs to (empty on the first register frame). |
| register | [Register](#cloud-v1-agent-Register) |  | register is the first frame identifying the host; session_id empty. |
| stdout | [bytes](#bytes) |  | stdout is PTY standard-output bytes for the session. |
| stderr | [bytes](#bytes) |  | stderr is PTY standard-error bytes for the session. |
| exit | [ShellExit](#cloud-v1-agent-ShellExit) |  | exit reports the session&#39;s process ending. |






<a name="cloud-v1-agent-OpenShell"></a>

### OpenShell
OpenShell asks the agent to spawn a PTY for a new session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run_id | [string](#string) |  | run_id runs this shell in the context of a run (for audit/scoping); may be empty. |
| component_id | [string](#string) |  | component_id is the target component on the host, optional. |
| cols | [uint32](#uint32) |  | cols is the initial terminal width in columns. |
| rows | [uint32](#uint32) |  | rows is the initial terminal height in rows. |
| shell | [string](#string) |  | shell is the shell binary to launch (e.g. &#34;/bin/bash&#34;); empty -&gt; agent default. |






<a name="cloud-v1-agent-Register"></a>

### Register
Register is the agent&#39;s first AgentShellMsg, identifying which host it is.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| machine_id | [string](#string) |  | machine_id is the host identifier the control stream belongs to. |






<a name="cloud-v1-agent-ServerShellMsg"></a>

### ServerShellMsg
ServerShellMsg is server -&gt; agent over the control stream.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| session_id | [string](#string) |  | session_id is the session this frame belongs to (server-assigned on open). |
| open | [OpenShell](#cloud-v1-agent-OpenShell) |  | open requests spawning a new PTY session. |
| stdin | [bytes](#bytes) |  | stdin is keystroke input bytes for the session&#39;s PTY. |
| resize | [ShellResize](#cloud-v1-agent-ShellResize) |  | resize updates the session&#39;s terminal window size. |
| close | [google.protobuf.Empty](#google-protobuf-Empty) |  | close terminates the session. |






<a name="cloud-v1-agent-ShellExit"></a>

### ShellExit
ShellExit reports a session ending.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [int32](#int32) |  | code is the process exit code of the shell session. |
| error | [string](#string) |  | error is an optional error message describing why the session ended. |






<a name="cloud-v1-agent-ShellResize"></a>

### ShellResize
ShellResize updates the PTY window size.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| cols | [uint32](#uint32) |  | cols is the new terminal width in columns. |
| rows | [uint32](#uint32) |  | rows is the new terminal height in rows. |





 

 

 


<a name="cloud-v1-agent-AgentShellAgentService"></a>

### AgentShellAgentService
AgentShellAgentApi is dialed by the agent (not by users). Auth is the agent
token, NOT the iam RBAC interceptor — hence no (cloud.v1.iam.auth) annotation.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Connect | [AgentShellMsg](#cloud-v1-agent-AgentShellMsg) stream | [ServerShellMsg](#cloud-v1-agent-ServerShellMsg) stream | Connect opens the long-lived, multiplexed control stream. |

 



<a name="cloud_v1_api_admin-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/admin.proto



<a name="cloud-v1-api-PlatformSettings"></a>

### PlatformSettings
PlatformSettings is the GLOBAL (singleton) control-plane configuration, changed
only by the root admin. It is NOT tenant-scoped — there is one control plane and
one row.

server_addr is the single public control-plane base URL handed to every agent
(STROPPY_SERVER_ADDR) so it knows where to Poll/Report and fetch its binary (the
agent binary URL is DERIVED from server_addr &#43; the server&#39;s agent-binary
endpoint — no separate setting). Empty -&gt; the server derives a docker-host
fallback (host.docker.internal / bridge gateway) for local Docker runs.

PROCESS GATES (allow_* flags). The remaining fields are coarse, global
kill-switches that the service layer checks BEFORE any RBAC permission, to
open or close whole flows platform-wide. They sit ABOVE the role model: a
closed gate denies everyone EXCEPT platform admins (Account.is_admin), who
are never blocked by these flags. RBAC still applies on top once a gate is
open. Every flag is polarised so its zero value (false) is the SAFE, most
restrictive state — a fresh/empty PlatformSettings locks the platform down.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| server_addr | [string](#string) |  | server_addr is the public control-plane base URL, e.g. http://1.1.1.1:8080. |
| allow_self_registration | [bool](#bool) |  | allow_self_registration toggles the PUBLIC self-signup endpoint (IamAPI.Register). When false (default) there is no open registration: accounts can only be created by a platform admin via CreateAccount. Flip it true to let anyone sign themselves up. |
| allow_member_tenant_creation | [bool](#bool) |  | allow_member_tenant_creation governs who may create tenants. When false (default) only platform admins may call CreateTenant, regardless of any RESOURCE_TENANT/ACTION_CREATE grant. When true, the normal RBAC permission check applies and ordinary members can spin up tenants. |





 

 

 

 



<a name="cloud_v1_api_agent_shell-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/agent_shell.proto



<a name="cloud-v1-api-ShellClientFrame"></a>

### ShellClientFrame
ShellClientFrame is admin -&gt; server. The first frame MUST be `start`.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| start | [ShellStart](#cloud-v1-api-ShellStart) |  | start opens the session (required first frame). |
| stdin | [bytes](#bytes) |  | stdin carries raw keystrokes typed into the terminal. |
| resize | [cloud.v1.agent.ShellResize](#cloud-v1-agent-ShellResize) |  | resize updates the terminal dimensions mid-session. |
| close | [google.protobuf.Empty](#google-protobuf-Empty) |  | close requests an orderly shutdown of the session. |






<a name="cloud-v1-api-ShellServerFrame"></a>

### ShellServerFrame
ShellServerFrame is server -&gt; admin.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| stdout | [bytes](#bytes) |  | stdout carries the terminal&#39;s standard-output bytes. |
| stderr | [bytes](#bytes) |  | stderr carries the terminal&#39;s standard-error bytes. |
| exit | [cloud.v1.agent.ShellExit](#cloud-v1-agent-ShellExit) |  | exit signals the shell process terminated (with its exit status). |






<a name="cloud-v1-api-ShellStart"></a>

### ShellStart
ShellStart is the REQUIRED first client frame: it picks the target and opens.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the session to one tenant; the streaming auth interceptor reads it from this first frame. |
| run_id | [string](#string) |  | run_id is the run context for audit/scoping (optional but recommended). |
| machine_id | [string](#string) |  | machine_id is the target host. Required. |
| component_id | [string](#string) |  | component_id is an optional target component on the host. |
| cols | [uint32](#uint32) |  | cols is the initial terminal width in columns. |
| rows | [uint32](#uint32) |  | rows is the initial terminal height in rows. |
| shell | [string](#string) |  | shell is the shell to launch; empty -&gt; agent default. |





 

 

 


<a name="cloud-v1-api-AgentShellService"></a>

### AgentShellService
AgentShellService bridges an admin websocket to an agent&#39;s grpc control
stream, exposing one interactive reverse-shell session per OpenShell call.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| OpenShell | [ShellClientFrame](#cloud-v1-api-ShellClientFrame) stream | [ShellServerFrame](#cloud-v1-api-ShellServerFrame) stream | OpenShell attaches an interactive terminal to an agent. First client frame must be ShellStart. High-privilege: RESOURCE_AGENT_SHELL &#43; audit. |

 



<a name="cloud_v1_api_compare-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/compare.proto



<a name="cloud-v1-api-CompareRunsRequest"></a>

### CompareRunsRequest
CompareRunsRequest asks for a side-by-side comparison of two or more runs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request; all runs must belong to this tenant. |
| run_ids | [string](#string) | repeated | run_ids are the runs to compare in display order; run_ids[0] is the baseline. |






<a name="cloud-v1-api-CompareRunsResponse"></a>

### CompareRunsResponse
CompareRunsResponse wraps the assembled comparison view.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| view | [CompareView](#cloud-v1-api-CompareView) |  | view is the side-by-side comparison (columns &#43; metric diff). |






<a name="cloud-v1-api-CompareView"></a>

### CompareView
CompareView is the full side-by-side comparison payload: one config column
per run plus the per-metric diff across all of them.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| columns | [RunColumn](#cloud-v1-api-RunColumn) | repeated | columns are the per-run config columns, aligned 1:1 with the request run_ids; columns[0] is the baseline. |
| metrics | [cloud.v1.monitor.Comparison](#cloud-v1-monitor-Comparison) |  | metrics is the per-metric diff across the runs (baseline = run_ids[0]). |






<a name="cloud-v1-api-RunColumn"></a>

### RunColumn
RunColumn is one run&#39;s descriptor in the comparison (config, not metrics).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run_id | [string](#string) |  | run_id is the test run this column describes. |
| name | [string](#string) |  | name is the run&#39;s display name. |
| status | [cloud.v1.common.Status](#cloud-v1-common-Status) |  | status is the run&#39;s lifecycle/terminal status. |
| db_kind | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) |  | db_kind is the database engine the run targeted. |
| db_name | [string](#string) |  | db_name is the database/preset name used by the run. |
| workload_name | [string](#string) |  | workload_name is the workload/preset the run executed. |
| stroppy_version | [string](#string) |  | stroppy_version is the stroppy engine version used. |
| provider | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) |  | provider is the deployment/cloud provider the run ran on. |
| topology_label | [string](#string) |  | topology_label is a human-readable summary of the cluster topology. |
| node_count | [uint32](#uint32) |  | node_count is the number of nodes in the run&#39;s topology. |
| started_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_at is when the run began (server clock). |
| duration | [google.protobuf.Duration](#google-protobuf-Duration) |  | duration is how long the run took. |





 

 

 


<a name="cloud-v1-api-CompareService"></a>

### CompareService
CompareService serves the side-by-side comparison page over N test runs.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CompareRuns | [CompareRunsRequest](#cloud-v1-api-CompareRunsRequest) | [CompareRunsResponse](#cloud-v1-api-CompareRunsResponse) | CompareRuns returns each run&#39;s config column plus the per-metric diff baselined on run_ids[0]. Read-only; gated by RESOURCE_TEST_RUN/READ. |

 



<a name="cloud_v1_api_favorite-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/favorite.proto



<a name="cloud-v1-api-AddFavoriteRequest"></a>

### AddFavoriteRequest
AddFavorite marks (kind, target_id) as a favorite of the caller. Idempotent:
favoriting an already-favorited row is a no-op (returns the existing record).
The server validates the target exists and is in the caller&#39;s tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the favorite; favorites are tenant-local and personal. |
| kind | [cloud.v1.common.FavoriteKind](#cloud-v1-common-FavoriteKind) |  | kind is the favoritable resource type (must be a defined, non-zero kind). |
| target_id | [string](#string) |  | target_id is the id of the row being favorited, within `kind`. |






<a name="cloud-v1-api-AddFavoriteResponse"></a>

### AddFavoriteResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| favorite | [cloud.v1.models.FavoriteRecord](#cloud-v1-models-FavoriteRecord) |  | favorite is the resulting (or pre-existing) favorite join record. |






<a name="cloud-v1-api-ListFavoritesRequest"></a>

### ListFavoritesRequest
ListFavorites returns the caller&#39;s favorites, optionally narrowed to one kind.
This lists the raw join rows; to list the favorited ENTITIES themselves, use
the target resource&#39;s List with EntityFilter.favorites_only = true instead.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the listing to the caller&#39;s tenant. |
| kind | [cloud.v1.common.FavoriteKind](#cloud-v1-common-FavoriteKind) |  | kind narrows to one kind; UNSPECIFIED = all kinds. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page is the pagination cursor/size. |






<a name="cloud-v1-api-ListFavoritesResponse"></a>

### ListFavoritesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| favorites | [cloud.v1.models.FavoriteRecord](#cloud-v1-models-FavoriteRecord) | repeated | favorites are the caller&#39;s favorite join rows for this page. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-RemoveFavoriteRequest"></a>

### RemoveFavoriteRequest
RemoveFavorite unmarks (kind, target_id). Idempotent: removing an absent
favorite is a no-op.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the favorite to the caller&#39;s tenant. |
| kind | [cloud.v1.common.FavoriteKind](#cloud-v1-common-FavoriteKind) |  | kind is the favoritable resource type (must be a defined, non-zero kind). |
| target_id | [string](#string) |  | target_id is the id of the row to unfavorite, within `kind`. |






<a name="cloud-v1-api-RemoveFavoriteResponse"></a>

### RemoveFavoriteResponse
RemoveFavoriteResponse is empty; success is signalled by the absence of error.





 

 

 


<a name="cloud-v1-api-FavoriteService"></a>

### FavoriteService
FavoriteService is the per-user favorite API covering every favoritable kind.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| AddFavorite | [AddFavoriteRequest](#cloud-v1-api-AddFavoriteRequest) | [AddFavoriteResponse](#cloud-v1-api-AddFavoriteResponse) | AddFavorite is idempotent. |
| RemoveFavorite | [RemoveFavoriteRequest](#cloud-v1-api-RemoveFavoriteRequest) | [RemoveFavoriteResponse](#cloud-v1-api-RemoveFavoriteResponse) | RemoveFavorite is idempotent. |
| ListFavorites | [ListFavoritesRequest](#cloud-v1-api-ListFavoritesRequest) | [ListFavoritesResponse](#cloud-v1-api-ListFavoritesResponse) | ListFavorites returns the caller&#39;s favorite join rows, optionally narrowed to one kind. Read-only. |

 



<a name="cloud_v1_api_iam-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/iam.proto



<a name="cloud-v1-api-CatalogEntry"></a>

### CatalogEntry
CatalogEntry is one grantable Permission plus a human label for the UI.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| permission | [cloud.v1.iam.Permission](#cloud-v1-iam-Permission) |  | permission is one grantable {resource, action} pair. |
| label | [string](#string) |  | label is a display string for the role editor, e.g. &#34;Create role&#34;. |






<a name="cloud-v1-api-ChangePasswordRequest"></a>

### ChangePasswordRequest
ChangePasswordRequest is the authenticated self-service rotation: the caller
proves knowledge of old_password and sets new_password. Always operates on
the caller&#39;s own account (from the token). Not idempotent — a replay fails
once old_password no longer matches.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| old_password | [string](#string) |  | old_password is the caller&#39;s current password, verified before the change. |
| new_password | [string](#string) |  | new_password is the replacement password to set. |






<a name="cloud-v1-api-ChangePasswordResponse"></a>

### ChangePasswordResponse
ChangePasswordResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-CompleteSSORequest"></a>

### CompleteSSORequest
CompleteSSORequest is the OIDC callback target. The IdP redirects the user
back with code &#43; state as query params; the http layer maps them here. The
server validates state, exchanges code for the IdP tokens, reads the subject,
resolves (or JIT-provisions, per the provider) the Account, and mints our own
TokenPair.

The OIDC nonce is NOT carried here: the server generates it alongside state &#43;
PKCE in StartSSO, stashes it server-side, and validates the id_token&#39;s nonce
claim during the code exchange — clients never see it.

HTTP NOTE. This RPC returns the TokenPair as JSON. Since the IdP redirects a
BROWSER to the callback, the http layer wrapping this RPC is responsible for
turning that response into a browser-friendly outcome (e.g. a 302 to the SPA
with the tokens, or a Set-Cookie) rather than rendering raw JSON. That same
layer also maps the callback route&#39;s &lt;slug&gt; (/auth/sso/&lt;slug&gt;/callback) onto
the provider_id field below — the RPC keys on id, the URL on slug.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider_id | [string](#string) |  | provider_id is the OIDC provider this callback belongs to (URL maps slug -&gt; id). |
| code | [string](#string) |  | code is the IdP authorization code to exchange for tokens. |
| state | [string](#string) |  | state is the CSRF token from StartSSO, validated against the stashed value. |






<a name="cloud-v1-api-CompleteSSOResponse"></a>

### CompleteSSOResponse
CompleteSSOResponse returns our own TokenPair for the resolved account.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tokens | [TokenPair](#cloud-v1-api-TokenPair) |  | tokens is the minted access &#43; refresh credential pair. |






<a name="cloud-v1-api-ConfirmPasswordResetRequest"></a>

### ConfirmPasswordResetRequest
ConfirmPasswordResetRequest completes the forgot-password flow: it consumes
the emailed token and sets new_password. PUBLIC; the token is the credential.
Not idempotent — the token is single-use.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [string](#string) |  | token is the single-use emailed reset token (the credential). |
| new_password | [string](#string) |  | new_password is the replacement password to set. |






<a name="cloud-v1-api-ConfirmPasswordResetResponse"></a>

### ConfirmPasswordResetResponse
ConfirmPasswordResetResponse is empty; success is signalled by the absence of
error.






<a name="cloud-v1-api-CreateAccountRequest"></a>

### CreateAccountRequest
CreateAccountRequest provisions a new global identity (the admin-only path;
the public path is RegisterRequest). The password is the only secret accepted
here; it is hashed and stored by the auth subsystem and never echoed back on
the returned Account.

An account needs at least ONE way to authenticate: the service rejects a
request with neither password nor link, since that yields an account no one
can ever log into. Supply a password, a link, or both.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| email | [string](#string) |  | email is the new account&#39;s contact &#43; login address (format-validated). |
| nickname | [string](#string) |  | nickname is the URL/handle-safe display handle (also an alternate login). |
| password | [string](#string) | optional | password is optional: omit it to create an SSO-only account that signs in exclusively through a linked external identity (see link). |
| is_admin | [bool](#bool) |  | is_admin may only be set by an existing platform admin; ignored otherwise. |
| link | [ExternalIdentityLink](#cloud-v1-api-ExternalIdentityLink) |  | link, when set, pre-links the new account to an external identity in the same call — the admin asserts the (provider, subject) binding so the user can log in via SSO immediately, without JIT provisioning. |






<a name="cloud-v1-api-CreateAccountResponse"></a>

### CreateAccountResponse
CreateAccountResponse returns the newly provisioned account (no secret).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account | [cloud.v1.iam.Account](#cloud-v1-iam-Account) |  | account is the created global identity (credentials never echoed). |






<a name="cloud-v1-api-CreateApiTokenRequest"></a>

### CreateApiTokenRequest
CreateApiTokenRequest mints a new token for account_id. The caller must be
that account or a platform admin (enforced server-side), so a user creates
their own and an admin can provision service tokens for others.

For a SERVICE token, permissions is the requested subset; the service rejects
any permission the owner does not currently hold (a token can never exceed its
owner). For a PERSONAL token, permissions MUST be empty — it inherits the
account&#39;s authority.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account_id | [string](#string) |  | account_id is the account the token authenticates as. |
| name | [string](#string) |  | name is a human-readable label for the token. |
| type | [cloud.v1.iam.ApiTokenType](#cloud-v1-iam-ApiTokenType) |  | type is PERSONAL (inherits the account&#39;s authority) or SERVICE (a capped subset); must be defined, non-zero. |
| permissions | [cloud.v1.iam.Permission](#cloud-v1-iam-Permission) | repeated | permissions is the requested grant for a SERVICE token; leave empty for a PERSONAL token. |
| ttl | [google.protobuf.Duration](#google-protobuf-Duration) |  | ttl is the optional lifetime from creation. Omit (or zero) for a token that never expires. |






<a name="cloud-v1-api-CreateApiTokenResponse"></a>

### CreateApiTokenResponse
CreateApiTokenResponse carries the created token&#39;s metadata AND the plaintext
secret. The secret is shown only here and never again — the caller must store
it now.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [cloud.v1.iam.ApiToken](#cloud-v1-iam-ApiToken) |  | token is the created token&#39;s metadata (no secret). |
| secret | [string](#string) |  | secret is the full plaintext token (prefix &#43; secret) to send as a bearer credential. Returned once; the server keeps only its hash. |






<a name="cloud-v1-api-CreateIdentityProviderRequest"></a>

### CreateIdentityProviderRequest
CreateIdentityProviderRequest configures a new OIDC provider. client_secret
is write-only: accepted here, stored server-side, never returned on the
IdentityProvider. The provider is created ENABLED (IdentityProvider.disabled
defaults false); hide it later via UpdateIdentityProvider if needed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| slug | [string](#string) |  | slug is the url-safe key used in the SSO callback route. |
| display_name | [string](#string) |  | display_name is the login-button label shown to users. |
| issuer | [string](#string) |  | issuer is the OIDC issuer URL (https, used for discovery). |
| client_id | [string](#string) |  | client_id is the OAuth client identifier registered at the IdP. |
| client_secret | [string](#string) |  | client_secret is the OAuth client secret; write-only (never returned). |
| scopes | [string](#string) | repeated | scopes are the OIDC scopes to request at authorize time. |
| allowed_domains | [string](#string) | repeated | allowed_domains restricts which email domains may sign in via this IdP. |
| auto_provision | [bool](#bool) |  | auto_provision enables JIT account creation for first-time SSO users. |






<a name="cloud-v1-api-CreateIdentityProviderResponse"></a>

### CreateIdentityProviderResponse
CreateIdentityProviderResponse returns the created provider (no secret).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider | [cloud.v1.iam.IdentityProvider](#cloud-v1-iam-IdentityProvider) |  | provider is the newly created (enabled) OIDC provider config. |






<a name="cloud-v1-api-CreateMembershipRequest"></a>

### CreateMembershipRequest
CreateMembershipRequest adds an account to a tenant with a set of roles — the
&#34;invite/add member&#34; operation. Referenced roles must be SCOPE_TENANT roles of
the same tenant_id or SCOPE_PLATFORM roles.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account_id | [string](#string) |  | account_id is the account being added to the tenant. |
| tenant_id | [string](#string) |  | tenant_id is the tenant the account joins. |
| role_ids | [string](#string) | repeated | role_ids are the granted roles (SCOPE_TENANT of this tenant, or SCOPE_PLATFORM); at least one is required. |






<a name="cloud-v1-api-CreateMembershipResponse"></a>

### CreateMembershipResponse
CreateMembershipResponse returns the created membership.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| membership | [cloud.v1.iam.Membership](#cloud-v1-iam-Membership) |  | membership is the newly created account-in-tenant grant. |






<a name="cloud-v1-api-CreateRoleRequest"></a>

### CreateRoleRequest
CreateRoleRequest defines a custom role. tenant_id MUST be set for
SCOPE_TENANT and empty for SCOPE_PLATFORM (enforced server-side). System
roles are seeded by the server and cannot be created here.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | name is the role&#39;s display name. |
| scope | [cloud.v1.iam.Scope](#cloud-v1-iam-Scope) |  | scope is SCOPE_TENANT or SCOPE_PLATFORM (must be defined, non-zero). |
| tenant_id | [string](#string) |  | tenant_id is required for SCOPE_TENANT, empty for SCOPE_PLATFORM. |
| permissions | [cloud.v1.iam.Permission](#cloud-v1-iam-Permission) | repeated | permissions is the role&#39;s granted permission set. |






<a name="cloud-v1-api-CreateRoleResponse"></a>

### CreateRoleResponse
CreateRoleResponse returns the created role.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| role | [cloud.v1.iam.Role](#cloud-v1-iam-Role) |  | role is the newly created custom role. |






<a name="cloud-v1-api-CreateTenantRequest"></a>

### CreateTenantRequest
CreateTenantRequest creates a workspace. The caller becomes owner and the
server seeds an owner Membership so the creator can immediately enter the
tenant. slug must be unique and url-safe (see iam/tenant.proto).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | name is the tenant&#39;s human-readable display name. |
| slug | [string](#string) |  | slug is the unique, url-safe routing key (/t/&lt;slug&gt;). |






<a name="cloud-v1-api-CreateTenantResponse"></a>

### CreateTenantResponse
CreateTenantResponse returns the created tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant | [cloud.v1.iam.Tenant](#cloud-v1-iam-Tenant) |  | tenant is the newly created workspace (caller seeded as owner). |






<a name="cloud-v1-api-DeleteAccountRequest"></a>

### DeleteAccountRequest
DeleteAccountRequest removes an account. The account may be referenced by
Memberships, ExternalIdentities, and owned Tenants (Tenant.owner_account_id);
the service layer decides the cascade: linked Memberships and
ExternalIdentities are removed with it, but a delete is REJECTED while the
account still owns any Tenant — transfer ownership first
(TransferTenantOwnership) so no tenant is orphaned.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the account to remove. |






<a name="cloud-v1-api-DeleteAccountResponse"></a>

### DeleteAccountResponse
DeleteAccountResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-DeleteIdentityProviderRequest"></a>

### DeleteIdentityProviderRequest
DeleteIdentityProviderRequest removes a provider by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the provider to remove. |






<a name="cloud-v1-api-DeleteIdentityProviderResponse"></a>

### DeleteIdentityProviderResponse
DeleteIdentityProviderResponse is empty; success is signalled by the absence
of error.






<a name="cloud-v1-api-DeleteMembershipRequest"></a>

### DeleteMembershipRequest
DeleteMembershipRequest removes a member from a tenant (revokes access).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the membership to remove (revokes the account&#39;s tenant access). |






<a name="cloud-v1-api-DeleteMembershipResponse"></a>

### DeleteMembershipResponse
DeleteMembershipResponse is empty; success is signalled by the absence of
error.






<a name="cloud-v1-api-DeleteRoleRequest"></a>

### DeleteRoleRequest
DeleteRoleRequest removes a role by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the role to remove. |






<a name="cloud-v1-api-DeleteRoleResponse"></a>

### DeleteRoleResponse
DeleteRoleResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-DeleteTenantRequest"></a>

### DeleteTenantRequest
DeleteTenantRequest removes a tenant by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the tenant to remove. |






<a name="cloud-v1-api-DeleteTenantResponse"></a>

### DeleteTenantResponse
DeleteTenantResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-ExternalIdentityLink"></a>

### ExternalIdentityLink
ExternalIdentityLink is an admin-asserted binding of a local account to an
IdP subject, used by CreateAccount and LinkExternalIdentity. The server
trusts the admin for the (provider_id, subject) pair; no SSO round-trip is
performed to verify it.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider_id | [string](#string) |  | provider_id is the IdentityProvider the subject belongs to. |
| subject | [string](#string) |  | subject is the IdP&#39;s stable subject identifier for the user. |
| email | [string](#string) |  | email is the address to record on the link (display / domain checks). |






<a name="cloud-v1-api-GetAccountRequest"></a>

### GetAccountRequest
GetAccountRequest fetches one account by id (RESOURCE_ACCOUNT/READ).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the account to fetch. |






<a name="cloud-v1-api-GetAccountResponse"></a>

### GetAccountResponse
GetAccountResponse returns the requested account.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account | [cloud.v1.iam.Account](#cloud-v1-iam-Account) |  | account is the fetched global identity. |






<a name="cloud-v1-api-GetIdentityProviderRequest"></a>

### GetIdentityProviderRequest
GetIdentityProviderRequest fetches one provider by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the provider to fetch. |






<a name="cloud-v1-api-GetIdentityProviderResponse"></a>

### GetIdentityProviderResponse
GetIdentityProviderResponse returns the requested provider.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider | [cloud.v1.iam.IdentityProvider](#cloud-v1-iam-IdentityProvider) |  | provider is the fetched OIDC provider config (no secret). |






<a name="cloud-v1-api-GetMembershipRequest"></a>

### GetMembershipRequest
GetMembershipRequest fetches one membership by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the membership to fetch. |






<a name="cloud-v1-api-GetMembershipResponse"></a>

### GetMembershipResponse
GetMembershipResponse returns the requested membership.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| membership | [cloud.v1.iam.Membership](#cloud-v1-iam-Membership) |  | membership is the fetched account-in-tenant grant. |






<a name="cloud-v1-api-GetMyAccountRequest"></a>

### GetMyAccountRequest
GetMyAccountRequest takes no arguments: it returns the CALLER&#39;s own Account,
resolved from the token subject. This is the self-profile endpoint — any
authenticated account may read itself without holding RESOURCE_ACCOUNT/READ.






<a name="cloud-v1-api-GetMyAccountResponse"></a>

### GetMyAccountResponse
GetMyAccountResponse returns the caller&#39;s own account.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account | [cloud.v1.iam.Account](#cloud-v1-iam-Account) |  | account is the caller&#39;s own global identity. |






<a name="cloud-v1-api-GetMyPermissionsRequest"></a>

### GetMyPermissionsRequest
GetMyPermissionsRequest resolves the CALLER&#39;s effective Permissions in one
tenant — the live union the gate computes per request, exposed so a UI can
show/hide controls. The account comes from the token; tenant from the ref.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id is the tenant to resolve the caller&#39;s effective permissions in. |






<a name="cloud-v1-api-GetMyPermissionsResponse"></a>

### GetMyPermissionsResponse
GetMyPermissionsResponse returns the caller&#39;s effective permissions.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| permissions | [cloud.v1.iam.Permission](#cloud-v1-iam-Permission) | repeated | permissions is the live union the gate computes for the caller in the tenant. |






<a name="cloud-v1-api-GetRoleRequest"></a>

### GetRoleRequest
GetRoleRequest fetches one role by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the role to fetch. |






<a name="cloud-v1-api-GetRoleResponse"></a>

### GetRoleResponse
GetRoleResponse returns the requested role.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| role | [cloud.v1.iam.Role](#cloud-v1-iam-Role) |  | role is the fetched role. |






<a name="cloud-v1-api-GetTenantRequest"></a>

### GetTenantRequest
GetTenantRequest fetches one tenant by EITHER id or slug (oneof). Slug lookup
is the routing path: the gate resolves /t/&lt;slug&gt; to a Tenant via this.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the tenant&#39;s stable identifier. |
| slug | [string](#string) |  | slug is the url-safe routing key (the /t/&lt;slug&gt; resolve path). |






<a name="cloud-v1-api-GetTenantResponse"></a>

### GetTenantResponse
GetTenantResponse returns the requested tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant | [cloud.v1.iam.Tenant](#cloud-v1-iam-Tenant) |  | tenant is the fetched workspace. |






<a name="cloud-v1-api-LeaveTenantRequest"></a>

### LeaveTenantRequest
LeaveTenantRequest is the authenticated self-service exit: the CALLER drops
their own Membership in the named tenant (the GitHub &#34;leave org&#34; action),
distinct from an admin removing someone else (DeleteMembership). The owner
MUST transfer ownership first (TransferTenantOwnership); the service rejects
an owner trying to leave. Idempotent — leaving a tenant you are not in is a
no-op.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id is the tenant the caller is leaving. |






<a name="cloud-v1-api-LeaveTenantResponse"></a>

### LeaveTenantResponse
LeaveTenantResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-LinkExternalIdentityRequest"></a>

### LinkExternalIdentityRequest
LinkExternalIdentityRequest binds an existing account to an IdP subject
(admin-asserted, like CreateAccount.link). Idempotent on (provider, subject).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account_id | [string](#string) |  | account_id is the existing account to bind to the IdP subject. |
| link | [ExternalIdentityLink](#cloud-v1-api-ExternalIdentityLink) |  | link is the admin-asserted (provider, subject) binding to record. |






<a name="cloud-v1-api-LinkExternalIdentityResponse"></a>

### LinkExternalIdentityResponse
LinkExternalIdentityResponse returns the created (or existing) link.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| identity | [cloud.v1.iam.ExternalIdentity](#cloud-v1-iam-ExternalIdentity) |  | identity is the resulting external-identity link. |






<a name="cloud-v1-api-ListAccountsRequest"></a>

### ListAccountsRequest
ListAccountsRequest is platform-scoped (admin only).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| page_size | [uint32](#uint32) |  | page_size caps returned rows; 0 -&gt; server default. |
| page_token | [string](#string) |  | page_token is the opaque cursor from a previous response. |






<a name="cloud-v1-api-ListAccountsResponse"></a>

### ListAccountsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| accounts | [cloud.v1.iam.Account](#cloud-v1-iam-Account) | repeated | accounts is this page of global identities. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-ListApiTokensRequest"></a>

### ListApiTokensRequest
ListApiTokensRequest lists the tokens of one account. The caller must be that
account or a platform admin (enforced server-side) — the &#34;my tokens&#34; view as
well as the admin one. Secrets are never included.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account_id | [string](#string) |  | account_id is the account whose tokens to list. |






<a name="cloud-v1-api-ListApiTokensResponse"></a>

### ListApiTokensResponse
ListApiTokensResponse returns the account&#39;s token metadata (no secrets).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tokens | [cloud.v1.iam.ApiToken](#cloud-v1-iam-ApiToken) | repeated | tokens is the account&#39;s tokens (metadata only). |






<a name="cloud-v1-api-ListExternalIdentitiesRequest"></a>

### ListExternalIdentitiesRequest
ListExternalIdentitiesRequest lists the identities linked to one account. The
caller must be that account or a platform admin (enforced server-side) — this
is the &#34;my linked logins&#34; view as well as the admin one.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account_id | [string](#string) |  | account_id is the account whose linked identities to list. |






<a name="cloud-v1-api-ListExternalIdentitiesResponse"></a>

### ListExternalIdentitiesResponse
ListExternalIdentitiesResponse returns the account&#39;s linked identities.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| identities | [cloud.v1.iam.ExternalIdentity](#cloud-v1-iam-ExternalIdentity) | repeated | identities are the external-identity links bound to the account. |






<a name="cloud-v1-api-ListIdentityProvidersRequest"></a>

### ListIdentityProvidersRequest
ListIdentityProvidersRequest is PUBLIC (the login page needs the buttons
before anyone is authenticated). It returns only ENABLED providers (those
with IdentityProvider.disabled == false). The response is slim on purpose —
only what a button needs — so admin-only config (domains, secret) never leaks
here.






<a name="cloud-v1-api-ListIdentityProvidersResponse"></a>

### ListIdentityProvidersResponse
ListIdentityProvidersResponse returns the enabled providers&#39; login buttons.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| buttons | [SsoButton](#cloud-v1-api-SsoButton) | repeated | buttons are the enabled providers, slimmed to what a login button needs. |






<a name="cloud-v1-api-ListMembershipsRequest"></a>

### ListMembershipsRequest
ListMembershipsRequest lists all members of one tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id is the tenant whose members to list. |
| page_size | [uint32](#uint32) |  | page_size caps returned rows; 0 -&gt; server default. |
| page_token | [string](#string) |  | page_token is the opaque cursor from a previous response. |






<a name="cloud-v1-api-ListMembershipsResponse"></a>

### ListMembershipsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| memberships | [cloud.v1.iam.Membership](#cloud-v1-iam-Membership) | repeated | memberships is this page of the tenant&#39;s members. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-ListMyTenantsRequest"></a>

### ListMyTenantsRequest
ListMyTenantsRequest returns the tenants the CALLER is a member of — the data
behind the org switcher. No arguments: the account comes from the token.






<a name="cloud-v1-api-ListMyTenantsResponse"></a>

### ListMyTenantsResponse
ListMyTenantsResponse returns the caller&#39;s tenants (the org switcher data).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenants | [cloud.v1.iam.Tenant](#cloud-v1-iam-Tenant) | repeated | tenants are the workspaces the caller is a member of. |






<a name="cloud-v1-api-ListPermissionsRequest"></a>

### ListPermissionsRequest
ListPermissionsRequest takes no arguments. ListPermissions returns the
catalog of grantable Permissions, assembled by the server from the
(cloud.v1.iam.auth) annotations across ALL services in the build (not only
IamAPI), PLUS a synthesized {resource, ACTION_MANAGE} entry for every
Resource that appears — MANAGE is grantable on a role but never named in an
annotation, so it must be added explicitly. RESOURCE/ACTION pairs that no
annotation references (e.g. RESOURCE_ACCOUNT/ACTION_CREATE, which is
admin_only) are intentionally absent: you cannot grant what nothing checks.






<a name="cloud-v1-api-ListPermissionsResponse"></a>

### ListPermissionsResponse
ListPermissionsResponse returns the full grantable-permission catalog.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entries | [CatalogEntry](#cloud-v1-api-CatalogEntry) | repeated | entries are the grantable permissions, each with a UI label. |






<a name="cloud-v1-api-ListRolesRequest"></a>

### ListRolesRequest
ListRolesRequest lists roles visible to the caller. tenant_id filters to one
tenant&#39;s roles; empty lists the platform-scoped roles. System roles are
included and marked via iam.Role.is_system.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id filters to one tenant&#39;s roles; empty lists platform-scoped roles. |
| page_size | [uint32](#uint32) |  | page_size caps returned rows; 0 -&gt; server default. |
| page_token | [string](#string) |  | page_token is the opaque cursor from a previous response. |






<a name="cloud-v1-api-ListRolesResponse"></a>

### ListRolesResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| roles | [cloud.v1.iam.Role](#cloud-v1-iam-Role) | repeated | roles is this page of roles (system roles marked via Role.is_system). |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-LoginRequest"></a>

### LoginRequest
LoginRequest authenticates by login &#43; password. login accepts EITHER a
nickname OR an email; the server resolves which by format. No separate flag:
&#34;alice&#34; and &#34;alice@example.com&#34; both go in login.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| login | [string](#string) |  | login is the account&#39;s nickname or email. |
| password | [string](#string) |  | password is the plaintext password, verified against the stored hash. |






<a name="cloud-v1-api-LoginResponse"></a>

### LoginResponse
LoginResponse returns the issued credential pair on a successful Login.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tokens | [TokenPair](#cloud-v1-api-TokenPair) |  | tokens is the freshly minted access &#43; refresh credential pair. |






<a name="cloud-v1-api-LogoutRequest"></a>

### LogoutRequest
LogoutRequest revokes the server-side session for the given refresh_token.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| refresh_token | [string](#string) |  | refresh_token identifies the server-side session to revoke. |






<a name="cloud-v1-api-LogoutResponse"></a>

### LogoutResponse
LogoutResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-RefreshRequest"></a>

### RefreshRequest
RefreshRequest exchanges a valid refresh_token for a fresh TokenPair. The
presented refresh_token is consumed (single-use); the response carries a new
rotated refresh_token. Travels in the body, not a cookie.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| refresh_token | [string](#string) |  | refresh_token is the single-use token to exchange; consumed on success. |






<a name="cloud-v1-api-RefreshResponse"></a>

### RefreshResponse
RefreshResponse returns the rotated credential pair (new refresh_token).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tokens | [TokenPair](#cloud-v1-api-TokenPair) |  | tokens is the new pair; persist the new refresh_token for the next Refresh. |






<a name="cloud-v1-api-RegisterRequest"></a>

### RegisterRequest
RegisterRequest is PUBLIC self-signup. It is gated by
PlatformSettings.allow_self_registration: when that flag is false the server
rejects this call and accounts can only be created by an admin via
CreateAccount. There is deliberately no is_admin field — a self-registered
account is never a platform admin.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| email | [string](#string) |  | email is the new account&#39;s contact &#43; login address (format-validated). |
| nickname | [string](#string) |  | nickname is the URL/handle-safe display handle (also an alternate login). |
| password | [string](#string) |  | password is the plaintext password to set; hashed and stored server-side. |






<a name="cloud-v1-api-RegisterResponse"></a>

### RegisterResponse
RegisterResponse auto-logs-in the new account by returning a TokenPair, so a
successful signup needs no follow-up Login. The account&#39;s email starts
UNVERIFIED (Account.email_verified == false); the server dispatches a
verification token through its Notifier (see VerifyEmailRequest). Login is not
blocked on verification — it is surfaced for the UI to nudge the user.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tokens | [TokenPair](#cloud-v1-api-TokenPair) |  | tokens auto-logs-in the new account (no follow-up Login needed). |






<a name="cloud-v1-api-RequestPasswordResetRequest"></a>

### RequestPasswordResetRequest
RequestPasswordResetRequest starts the PUBLIC forgot-password flow: the
server mints a single-use, time-bounded reset token and delivers it to the
account&#39;s email via its Notifier. The response is ALWAYS empty/success
regardless of whether the email exists — never leak account existence.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| email | [string](#string) |  | email is the address to send the reset token to (existence never leaked). |






<a name="cloud-v1-api-RequestPasswordResetResponse"></a>

### RequestPasswordResetResponse
RequestPasswordResetResponse is always empty/success (no account-existence
leak).






<a name="cloud-v1-api-ResendVerificationRequest"></a>

### ResendVerificationRequest
ResendVerificationRequest re-dispatches a fresh verification token to the
CALLER&#39;s own (still-unverified) email via the Notifier. Authenticated; no
arguments — the account comes from the token.






<a name="cloud-v1-api-ResendVerificationResponse"></a>

### ResendVerificationResponse
ResendVerificationResponse is empty; success is signalled by the absence of
error.






<a name="cloud-v1-api-ResetPasswordRequest"></a>

### ResetPasswordRequest
ResetPasswordRequest is the admin override: a platform admin sets a new
password for any account without knowing the old one (e.g. account recovery).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account_id | [string](#string) |  | account_id is the account whose password the admin is resetting. |
| new_password | [string](#string) |  | new_password is the replacement password to set. |






<a name="cloud-v1-api-ResetPasswordResponse"></a>

### ResetPasswordResponse
ResetPasswordResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-RevokeApiTokenRequest"></a>

### RevokeApiTokenRequest
RevokeApiTokenRequest permanently disables one token by id. The caller must
own the token&#39;s account or be a platform admin (enforced server-side).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the token to permanently disable. |






<a name="cloud-v1-api-RevokeApiTokenResponse"></a>

### RevokeApiTokenResponse
RevokeApiTokenResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-SsoButton"></a>

### SsoButton
SsoButton is the public, login-page view of an enabled provider.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the provider id to pass to StartSSO. |
| slug | [string](#string) |  | slug is the provider&#39;s url-safe key. |
| display_name | [string](#string) |  | display_name is the button label. |






<a name="cloud-v1-api-StartSSORequest"></a>

### StartSSORequest
StartSSORequest begins the OIDC authorization-code flow for one provider. The
server builds the IdP authorize URL (with a freshly generated state &#43; PKCE
challenge it stashes server-side) and returns it; the client redirects the
user there.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider_id | [string](#string) |  | provider_id is the OIDC provider to begin the authorization-code flow for. |






<a name="cloud-v1-api-StartSSOResponse"></a>

### StartSSOResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| redirect_url | [string](#string) |  | redirect_url is the IdP authorize endpoint the caller must send the user to. |
| state | [string](#string) |  | state is the opaque CSRF token echoed back to CompleteSSO; the server also keeps it to validate the callback. |






<a name="cloud-v1-api-TokenPair"></a>

### TokenPair
TokenPair is the result of a successful Register, Login or Refresh. Both are
bearer credentials.

access_token is short-lived and sent on every API call (Authorization:
Bearer). It is TENANT-AGNOSTIC (see iam/claims.proto) — one token works
across every tenant the account can reach, so switching tenant never re-mints
it.

refresh_token is long-lived, single-use, and ROTATED on every Refresh: each
Refresh invalidates the presented refresh_token and returns a new one, so the
caller MUST persist the new refresh_token or the next Refresh fails. The
server keeps the refresh token server-side so Logout can revoke it.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| access_token | [string](#string) |  | access_token is the short-lived, tenant-agnostic bearer token. |
| refresh_token | [string](#string) |  | refresh_token is the long-lived, single-use, rotated token. |
| access_expires_in | [google.protobuf.Duration](#google-protobuf-Duration) |  | access_expires_in is the access_token lifetime from issuance. |
| refresh_expires_in | [google.protobuf.Duration](#google-protobuf-Duration) |  | refresh_expires_in is the refresh_token lifetime from issuance. |






<a name="cloud-v1-api-TransferTenantOwnershipRequest"></a>

### TransferTenantOwnershipRequest
TransferTenantOwnershipRequest reassigns Tenant.owner_account_id to another
account, which MUST already be a member of the tenant. Ownership is the only
way to change owner_account_id (UpdateTenant cannot), keeping the transfer an
explicit, auditable action.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id is the tenant whose ownership is being reassigned. |
| new_owner_account_id | [string](#string) |  | new_owner_account_id is the new owner; MUST already be a member. |






<a name="cloud-v1-api-TransferTenantOwnershipResponse"></a>

### TransferTenantOwnershipResponse
TransferTenantOwnershipResponse returns the tenant after the transfer.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant | [cloud.v1.iam.Tenant](#cloud-v1-iam-Tenant) |  | tenant is the workspace with its new owner_account_id. |






<a name="cloud-v1-api-UnlinkExternalIdentityRequest"></a>

### UnlinkExternalIdentityRequest
UnlinkExternalIdentityRequest removes one external identity by its id,
revoking SSO login through it (the Account and other links remain). The
caller must own the identity&#39;s account or be a platform admin (enforced
server-side).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the external-identity link to remove. |






<a name="cloud-v1-api-UnlinkExternalIdentityResponse"></a>

### UnlinkExternalIdentityResponse
UnlinkExternalIdentityResponse is empty; success is signalled by the absence
of error.






<a name="cloud-v1-api-UpdateAccountRequest"></a>

### UpdateAccountRequest
UpdateAccountRequest mutates the named account. Only email/nickname are
editable here; password rotation and is_admin changes are separate,
privilege-gated operations.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the account to mutate. |
| email | [string](#string) | optional | email, when present, replaces the account&#39;s contact &#43; login address. |
| nickname | [string](#string) | optional | nickname, when present, replaces the account&#39;s handle. |






<a name="cloud-v1-api-UpdateAccountResponse"></a>

### UpdateAccountResponse
UpdateAccountResponse returns the account after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| account | [cloud.v1.iam.Account](#cloud-v1-iam-Account) |  | account is the updated global identity. |






<a name="cloud-v1-api-UpdateIdentityProviderRequest"></a>

### UpdateIdentityProviderRequest
UpdateIdentityProviderRequest edits a provider. A present, non-empty
client_secret ROTATES the stored secret; an absent one leaves it unchanged.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the provider to edit. |
| display_name | [string](#string) | optional | display_name, when present, replaces the login-button label. |
| issuer | [string](#string) | optional | issuer, when present, replaces the OIDC issuer URL. |
| client_id | [string](#string) | optional | client_id, when present, replaces the OAuth client identifier. |
| client_secret | [string](#string) | optional | client_secret, when present and non-empty, ROTATES the stored secret; absent leaves it unchanged. |
| scopes | [string](#string) | repeated | scopes, when present, replaces the requested OIDC scopes. |
| allowed_domains | [string](#string) | repeated | allowed_domains, when present, replaces the email-domain allowlist. |
| auto_provision | [bool](#bool) | optional | auto_provision, when present, toggles JIT account creation. |
| disabled | [bool](#bool) | optional | disabled hides the provider (inverted polarity: false = enabled). See iam/sso.proto. |






<a name="cloud-v1-api-UpdateIdentityProviderResponse"></a>

### UpdateIdentityProviderResponse
UpdateIdentityProviderResponse returns the provider after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| provider | [cloud.v1.iam.IdentityProvider](#cloud-v1-iam-IdentityProvider) |  | provider is the updated OIDC provider config (no secret). |






<a name="cloud-v1-api-UpdateMembershipRequest"></a>

### UpdateMembershipRequest
UpdateMembershipRequest changes a member&#39;s granted roles. role_ids REPLACES
the existing set wholesale; an empty set is rejected (remove the membership
instead of leaving it role-less).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the membership to edit. |
| role_ids | [string](#string) | repeated | role_ids REPLACES the granted role set wholesale; an empty set is rejected. |






<a name="cloud-v1-api-UpdateMembershipResponse"></a>

### UpdateMembershipResponse
UpdateMembershipResponse returns the membership after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| membership | [cloud.v1.iam.Membership](#cloud-v1-iam-Membership) |  | membership is the updated grant. |






<a name="cloud-v1-api-UpdateRoleRequest"></a>

### UpdateRoleRequest
UpdateRoleRequest edits a custom role. System roles (is_system) are rejected.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the custom role to edit (system roles are rejected). |
| name | [string](#string) | optional | name, when present, replaces the role&#39;s display name. |
| permissions | [cloud.v1.iam.Permission](#cloud-v1-iam-Permission) | repeated | permissions, when present, REPLACES the role&#39;s permission set wholesale. |






<a name="cloud-v1-api-UpdateRoleResponse"></a>

### UpdateRoleResponse
UpdateRoleResponse returns the role after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| role | [cloud.v1.iam.Role](#cloud-v1-iam-Role) |  | role is the updated role. |






<a name="cloud-v1-api-UpdateTenantRequest"></a>

### UpdateTenantRequest
UpdateTenantRequest mutates name/slug. Renaming slug breaks old links.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | id is the tenant to mutate. |
| name | [string](#string) | optional | name, when present, replaces the display name. |
| slug | [string](#string) | optional | slug, when present, replaces the routing key (breaks old links). |






<a name="cloud-v1-api-UpdateTenantResponse"></a>

### UpdateTenantResponse
UpdateTenantResponse returns the tenant after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant | [cloud.v1.iam.Tenant](#cloud-v1-iam-Tenant) |  | tenant is the updated workspace. |






<a name="cloud-v1-api-VerifyEmailRequest"></a>

### VerifyEmailRequest
VerifyEmailRequest consumes an emailed verification token and flips the
target account&#39;s Account.email_verified to true. PUBLIC: the token is the
credential and the user may not be logged in when clicking the link.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [string](#string) |  | token is the single-use emailed verification token (the credential). |






<a name="cloud-v1-api-VerifyEmailResponse"></a>

### VerifyEmailResponse
VerifyEmailResponse is empty; success is signalled by the absence of error.





 

 

 


<a name="cloud-v1-api-IamService"></a>

### IamService
--- Auth (public: no bearer token) ---

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Register | [RegisterRequest](#cloud-v1-api-RegisterRequest) | [RegisterResponse](#cloud-v1-api-RegisterResponse) | Register is public self-signup, gated by PlatformSettings.allow_self_registration. Not idempotent: each call creates a new account. |
| Login | [LoginRequest](#cloud-v1-api-LoginRequest) | [LoginResponse](#cloud-v1-api-LoginResponse) | Login authenticates by nickname|email &#43; password and issues a TokenPair. Not idempotent: each call mints a new session/TokenPair. |
| Refresh | [RefreshRequest](#cloud-v1-api-RefreshRequest) | [RefreshResponse](#cloud-v1-api-RefreshResponse) | Refresh rotates a refresh_token into a new TokenPair. Not idempotent: the refresh_token is single-use; a replay fails. |
| Logout | [LogoutRequest](#cloud-v1-api-LogoutRequest) | [LogoutResponse](#cloud-v1-api-LogoutResponse) | Logout revokes the refresh_token&#39;s server-side session. Idempotent: revoking an already-revoked session is a no-op. |
| RequestPasswordReset | [RequestPasswordResetRequest](#cloud-v1-api-RequestPasswordResetRequest) | [RequestPasswordResetResponse](#cloud-v1-api-RequestPasswordResetResponse) | RequestPasswordReset starts the forgot-password flow (emails a token). Public. Always succeeds (no account-existence leak). |
| ConfirmPasswordReset | [ConfirmPasswordResetRequest](#cloud-v1-api-ConfirmPasswordResetRequest) | [ConfirmPasswordResetResponse](#cloud-v1-api-ConfirmPasswordResetResponse) | ConfirmPasswordReset consumes the emailed token and sets a new password. Public. Not idempotent: the token is single-use. |
| VerifyEmail | [VerifyEmailRequest](#cloud-v1-api-VerifyEmailRequest) | [VerifyEmailResponse](#cloud-v1-api-VerifyEmailResponse) | VerifyEmail consumes an emailed verification token. Public: the token is the credential and the user may be logged out. |
| CreateAccount | [CreateAccountRequest](#cloud-v1-api-CreateAccountRequest) | [CreateAccountResponse](#cloud-v1-api-CreateAccountResponse) | CreateAccount is the admin-only account path (the public path is Register). Not idempotent: each call creates a new account. |
| GetAccount | [GetAccountRequest](#cloud-v1-api-GetAccountRequest) | [GetAccountResponse](#cloud-v1-api-GetAccountResponse) | GetAccount fetches one account by id. Read-only. |
| GetMyAccount | [GetMyAccountRequest](#cloud-v1-api-GetMyAccountRequest) | [GetMyAccountResponse](#cloud-v1-api-GetMyAccountResponse) | GetMyAccount returns the caller&#39;s own profile — authenticated, no permission (self-read). |
| ListAccounts | [ListAccountsRequest](#cloud-v1-api-ListAccountsRequest) | [ListAccountsResponse](#cloud-v1-api-ListAccountsResponse) | ListAccounts lists accounts platform-wide. Read-only. |
| UpdateAccount | [UpdateAccountRequest](#cloud-v1-api-UpdateAccountRequest) | [UpdateAccountResponse](#cloud-v1-api-UpdateAccountResponse) | UpdateAccount is idempotent: a wholesale field set converges on retry. |
| DeleteAccount | [DeleteAccountRequest](#cloud-v1-api-DeleteAccountRequest) | [DeleteAccountResponse](#cloud-v1-api-DeleteAccountResponse) | DeleteAccount is idempotent: deleting an absent account is a no-op. |
| ChangePassword | [ChangePasswordRequest](#cloud-v1-api-ChangePasswordRequest) | [ChangePasswordResponse](#cloud-v1-api-ChangePasswordResponse) | ChangePassword is authenticated self-service (old &#43; new). Not idempotent. |
| ResetPassword | [ResetPasswordRequest](#cloud-v1-api-ResetPasswordRequest) | [ResetPasswordResponse](#cloud-v1-api-ResetPasswordResponse) | ResetPassword is the admin override (sets a new password for any account). Idempotent: setting the same password twice converges. |
| ResendVerification | [ResendVerificationRequest](#cloud-v1-api-ResendVerificationRequest) | [ResendVerificationResponse](#cloud-v1-api-ResendVerificationResponse) | ResendVerification re-sends the caller&#39;s own email verification token. Authenticated. |
| CreateTenant | [CreateTenantRequest](#cloud-v1-api-CreateTenantRequest) | [CreateTenantResponse](#cloud-v1-api-CreateTenantResponse) | CreateTenant is gated by PlatformSettings.allow_member_tenant_creation. Not idempotent: each call creates a new tenant. |
| GetTenant | [GetTenantRequest](#cloud-v1-api-GetTenantRequest) | [GetTenantResponse](#cloud-v1-api-GetTenantResponse) | GetTenant fetches by id or slug; slug is the routing-resolve path. |
| ListMyTenants | [ListMyTenantsRequest](#cloud-v1-api-ListMyTenantsRequest) | [ListMyTenantsResponse](#cloud-v1-api-ListMyTenantsResponse) | ListMyTenants returns the caller&#39;s tenants — authenticated, no permission. |
| UpdateTenant | [UpdateTenantRequest](#cloud-v1-api-UpdateTenantRequest) | [UpdateTenantResponse](#cloud-v1-api-UpdateTenantResponse) | UpdateTenant edits name/slug. Idempotent: a wholesale field set converges on retry. |
| DeleteTenant | [DeleteTenantRequest](#cloud-v1-api-DeleteTenantRequest) | [DeleteTenantResponse](#cloud-v1-api-DeleteTenantResponse) | DeleteTenant removes a tenant. Idempotent: deleting an absent tenant is a no-op. |
| TransferTenantOwnership | [TransferTenantOwnershipRequest](#cloud-v1-api-TransferTenantOwnershipRequest) | [TransferTenantOwnershipResponse](#cloud-v1-api-TransferTenantOwnershipResponse) | TransferTenantOwnership reassigns the owner. Idempotent: transferring to the current owner is a no-op. |
| LeaveTenant | [LeaveTenantRequest](#cloud-v1-api-LeaveTenantRequest) | [LeaveTenantResponse](#cloud-v1-api-LeaveTenantResponse) | LeaveTenant drops the caller&#39;s own membership — authenticated, no permission. Idempotent: leaving a tenant you are not in is a no-op. |
| CreateRole | [CreateRoleRequest](#cloud-v1-api-CreateRoleRequest) | [CreateRoleResponse](#cloud-v1-api-CreateRoleResponse) | CreateRole is not idempotent: each call creates a new role. |
| GetRole | [GetRoleRequest](#cloud-v1-api-GetRoleRequest) | [GetRoleResponse](#cloud-v1-api-GetRoleResponse) | GetRole fetches one role by id. Read-only. |
| ListRoles | [ListRolesRequest](#cloud-v1-api-ListRolesRequest) | [ListRolesResponse](#cloud-v1-api-ListRolesResponse) | ListRoles lists roles visible to the caller (tenant or platform scope). Read-only. |
| UpdateRole | [UpdateRoleRequest](#cloud-v1-api-UpdateRoleRequest) | [UpdateRoleResponse](#cloud-v1-api-UpdateRoleResponse) | UpdateRole is idempotent: permissions REPLACES wholesale, converging on retry. |
| DeleteRole | [DeleteRoleRequest](#cloud-v1-api-DeleteRoleRequest) | [DeleteRoleResponse](#cloud-v1-api-DeleteRoleResponse) | DeleteRole removes a custom role. Idempotent: deleting an absent role is a no-op. |
| CreateMembership | [CreateMembershipRequest](#cloud-v1-api-CreateMembershipRequest) | [CreateMembershipResponse](#cloud-v1-api-CreateMembershipResponse) | CreateMembership is not idempotent: each call creates a new membership row. |
| GetMembership | [GetMembershipRequest](#cloud-v1-api-GetMembershipRequest) | [GetMembershipResponse](#cloud-v1-api-GetMembershipResponse) | GetMembership fetches one membership by id. Read-only. |
| ListMemberships | [ListMembershipsRequest](#cloud-v1-api-ListMembershipsRequest) | [ListMembershipsResponse](#cloud-v1-api-ListMembershipsResponse) | ListMemberships lists one tenant&#39;s members. Read-only. |
| UpdateMembership | [UpdateMembershipRequest](#cloud-v1-api-UpdateMembershipRequest) | [UpdateMembershipResponse](#cloud-v1-api-UpdateMembershipResponse) | UpdateMembership is idempotent: role_ids REPLACES wholesale, converging on retry. |
| DeleteMembership | [DeleteMembershipRequest](#cloud-v1-api-DeleteMembershipRequest) | [DeleteMembershipResponse](#cloud-v1-api-DeleteMembershipResponse) | DeleteMembership removes a member from a tenant (revokes access). Idempotent: removing an absent membership is a no-op. |
| GetMyPermissions | [GetMyPermissionsRequest](#cloud-v1-api-GetMyPermissionsRequest) | [GetMyPermissionsResponse](#cloud-v1-api-GetMyPermissionsResponse) | GetMyPermissions resolves the caller&#39;s own permissions — authenticated, no permission. |
| ListPermissions | [ListPermissionsRequest](#cloud-v1-api-ListPermissionsRequest) | [ListPermissionsResponse](#cloud-v1-api-ListPermissionsResponse) | ListPermissions returns the grantable-permission catalog — authenticated, no permission. |
| CreateIdentityProvider | [CreateIdentityProviderRequest](#cloud-v1-api-CreateIdentityProviderRequest) | [CreateIdentityProviderResponse](#cloud-v1-api-CreateIdentityProviderResponse) | CreateIdentityProvider configures a new OIDC provider. Admin-only. Not idempotent: each call creates a new provider. |
| GetIdentityProvider | [GetIdentityProviderRequest](#cloud-v1-api-GetIdentityProviderRequest) | [GetIdentityProviderResponse](#cloud-v1-api-GetIdentityProviderResponse) | GetIdentityProvider fetches one provider by id. Admin-only, read-only. |
| UpdateIdentityProvider | [UpdateIdentityProviderRequest](#cloud-v1-api-UpdateIdentityProviderRequest) | [UpdateIdentityProviderResponse](#cloud-v1-api-UpdateIdentityProviderResponse) | UpdateIdentityProvider edits a provider (a present secret rotates it). Admin-only. Idempotent: a wholesale field set converges on retry. |
| DeleteIdentityProvider | [DeleteIdentityProviderRequest](#cloud-v1-api-DeleteIdentityProviderRequest) | [DeleteIdentityProviderResponse](#cloud-v1-api-DeleteIdentityProviderResponse) | DeleteIdentityProvider removes a provider. Admin-only. Idempotent: deleting an absent provider is a no-op. |
| ListIdentityProviders | [ListIdentityProvidersRequest](#cloud-v1-api-ListIdentityProvidersRequest) | [ListIdentityProvidersResponse](#cloud-v1-api-ListIdentityProvidersResponse) | ListIdentityProviders returns the login-page buttons. Public, read-only. |
| StartSSO | [StartSSORequest](#cloud-v1-api-StartSSORequest) | [StartSSOResponse](#cloud-v1-api-StartSSOResponse) | StartSSO returns the IdP authorize URL. Public. Not idempotent: each call mints fresh state &#43; PKCE stashed server-side. |
| CompleteSSO | [CompleteSSORequest](#cloud-v1-api-CompleteSSORequest) | [CompleteSSOResponse](#cloud-v1-api-CompleteSSOResponse) | CompleteSSO is the OIDC callback. Public. Not idempotent: the code is single-use and a replay fails. |
| LinkExternalIdentity | [LinkExternalIdentityRequest](#cloud-v1-api-LinkExternalIdentityRequest) | [LinkExternalIdentityResponse](#cloud-v1-api-LinkExternalIdentityResponse) | LinkExternalIdentity is admin-only (admin asserts the binding). Idempotent on (provider, subject). |
| UnlinkExternalIdentity | [UnlinkExternalIdentityRequest](#cloud-v1-api-UnlinkExternalIdentityRequest) | [UnlinkExternalIdentityResponse](#cloud-v1-api-UnlinkExternalIdentityResponse) | UnlinkExternalIdentity is authenticated; the service enforces account-owner-or-admin. Idempotent: unlinking an absent link is a no-op. |
| ListExternalIdentities | [ListExternalIdentitiesRequest](#cloud-v1-api-ListExternalIdentitiesRequest) | [ListExternalIdentitiesResponse](#cloud-v1-api-ListExternalIdentitiesResponse) | ListExternalIdentities is authenticated; the service enforces account-owner-or-admin (the &#34;my linked logins&#34; view). |
| CreateApiToken | [CreateApiTokenRequest](#cloud-v1-api-CreateApiTokenRequest) | [CreateApiTokenResponse](#cloud-v1-api-CreateApiTokenResponse) | CreateApiToken mints a programmatic credential; the service enforces account-owner-or-admin and caps a SERVICE token to the owner&#39;s permissions. Not idempotent: each call mints a new token &#43; secret. |
| ListApiTokens | [ListApiTokensRequest](#cloud-v1-api-ListApiTokensRequest) | [ListApiTokensResponse](#cloud-v1-api-ListApiTokensResponse) | ListApiTokens lists an account&#39;s tokens (metadata only). Authenticated; the service enforces account-owner-or-admin. |
| RevokeApiToken | [RevokeApiTokenRequest](#cloud-v1-api-RevokeApiTokenRequest) | [RevokeApiTokenResponse](#cloud-v1-api-RevokeApiTokenResponse) | RevokeApiToken disables a token. Authenticated; the service enforces account-owner-or-admin. Idempotent: revoking an absent/already-revoked token is a no-op. |

 



<a name="cloud_v1_api_package-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/package.proto



<a name="cloud-v1-api-CompleteUploadRequest"></a>

### CompleteUploadRequest
CompleteUpload finalizes after the client PUT the blob: the server verifies
size &#43; sha256 and flips the record to READY (or FAILED). Idempotent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the package&#39;s tenant. |
| id | [string](#string) |  | id is the package record being finalized. |






<a name="cloud-v1-api-CompleteUploadResponse"></a>

### CompleteUploadResponse
CompleteUploadResponse returns the record after verification (READY/FAILED).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| package | [cloud.v1.models.PackageRecord](#cloud-v1-models-PackageRecord) |  | package is the finalized record (now STATUS_READY or STATUS_FAILED). |






<a name="cloud-v1-api-CreatePackageUploadRequest"></a>

### CreatePackageUploadRequest
CreatePackageUploadRequest declares the package metadata and mints a presigned
upload; the blob is PUT directly to object storage afterward.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the package; packages are tenant-private. |
| name | [string](#string) |  | name is the package&#39;s display name. |
| format | [cloud.v1.models.PackageRecord.Format](#cloud-v1-models-PackageRecord-Format) |  | format is the package format (.deb / binary); must be defined, non-zero. |
| version | [string](#string) |  | version is the package version string. |
| target_db_kind | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) |  | target_db_kind is the database engine this package builds/installs. |
| os | [string](#string) |  | os is the target operating system the package is built for. |
| arch | [string](#string) |  | arch is the target CPU architecture the package is built for. |
| size_bytes | [uint64](#uint64) |  | size_bytes is the declared blob size; verified on CompleteUpload against the uploaded object. |
| sha256 | [string](#string) |  | sha256 is the declared blob hash; verified on CompleteUpload against the uploaded object. |






<a name="cloud-v1-api-CreatePackageUploadResponse"></a>

### CreatePackageUploadResponse
CreatePackageUploadResponse returns the pending record plus the presigned PUT
url the client uploads the blob to.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| package | [cloud.v1.models.PackageRecord](#cloud-v1-models-PackageRecord) |  | package is the created record (STATUS_UPLOADING). |
| upload_url | [string](#string) |  | upload_url is the presigned PUT url the client uploads the blob to. |
| upload_url_expires_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | upload_url_expires_at is when the upload url stops working. |






<a name="cloud-v1-api-DeletePackageRequest"></a>

### DeletePackageRequest
DeletePackageRequest removes one package by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the package&#39;s tenant. |
| id | [string](#string) |  | id is the package to remove. |






<a name="cloud-v1-api-DeletePackageResponse"></a>

### DeletePackageResponse
DeletePackageResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-GetPackageRequest"></a>

### GetPackageRequest
GetPackageRequest fetches one package by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the package&#39;s tenant. |
| id | [string](#string) |  | id is the package to fetch. |






<a name="cloud-v1-api-GetPackageResponse"></a>

### GetPackageResponse
GetPackageResponse returns the requested package.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| package | [cloud.v1.models.PackageRecord](#cloud-v1-models-PackageRecord) |  | package is the fetched package record. |






<a name="cloud-v1-api-ListPackagesRequest"></a>

### ListPackagesRequest
ListPackagesRequest lists a tenant&#39;s packages with filtering, sort and paging.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the listing to one tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter is the shared Entity-level filter (search, ids, time windows). |
| formats | [cloud.v1.models.PackageRecord.Format](#cloud-v1-models-PackageRecord-Format) | repeated | formats narrows to specific package formats (facet filter). |
| db_kinds | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) | repeated | db_kinds narrows to specific target database engines (facet filter). |
| sort | [cloud.v1.common.EntitySort](#cloud-v1-common-EntitySort) |  | sort is the ordering over the common Entity columns. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page is the pagination cursor/size. |






<a name="cloud-v1-api-ListPackagesResponse"></a>

### ListPackagesResponse
ListPackagesResponse returns one page of packages.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| packages | [cloud.v1.models.PackageRecord](#cloud-v1-models-PackageRecord) | repeated | packages is this page of package records. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |





 

 

 


<a name="cloud-v1-api-PackageService"></a>

### PackageService
PackageService is the tenant package registry (custom DB builds).

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreatePackageUpload | [CreatePackageUploadRequest](#cloud-v1-api-CreatePackageUploadRequest) | [CreatePackageUploadResponse](#cloud-v1-api-CreatePackageUploadResponse) | CreatePackageUpload mints a record &#43; presigned PUT url. Not idempotent. |
| CompleteUpload | [CompleteUploadRequest](#cloud-v1-api-CompleteUploadRequest) | [CompleteUploadResponse](#cloud-v1-api-CompleteUploadResponse) | CompleteUpload verifies size &#43; sha256 and flips the record to READY (or FAILED). Idempotent. |
| GetPackage | [GetPackageRequest](#cloud-v1-api-GetPackageRequest) | [GetPackageResponse](#cloud-v1-api-GetPackageResponse) | GetPackage fetches one package by id. Read-only. |
| ListPackages | [ListPackagesRequest](#cloud-v1-api-ListPackagesRequest) | [ListPackagesResponse](#cloud-v1-api-ListPackagesResponse) | ListPackages lists a tenant&#39;s packages. Read-only. |
| DeletePackage | [DeletePackageRequest](#cloud-v1-api-DeletePackageRequest) | [DeletePackageResponse](#cloud-v1-api-DeletePackageResponse) | DeletePackage is idempotent: deleting an absent package is a no-op. |

 



<a name="cloud_v1_api_preset-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/preset.proto



<a name="cloud-v1-api-CloneDatabasePresetRequest"></a>

### CloneDatabasePresetRequest
Clone copies a preset (typically a read-only system one) into a new editable
preset owned by the caller. The copy gets a fresh id, is_system = false and
the caller as author.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the source preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to copy. |
| name | [string](#string) |  | name is the optional name for the copy; empty -&gt; server derives one (e.g. &#34;&lt;name&gt; (copy)&#34;). |






<a name="cloud-v1-api-CloneDatabasePresetResponse"></a>

### CloneDatabasePresetResponse
CloneDatabasePresetResponse returns the new editable copy.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-DatabasePresetRecord) |  | preset is the cloned, editable database preset. |






<a name="cloud-v1-api-CloneTestPresetRequest"></a>

### CloneTestPresetRequest
Clone copies a preset (typically a read-only system one) into a new editable
preset owned by the caller. The copy gets a fresh id, is_system = false and
the caller as author.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the source preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to copy. |
| name | [string](#string) |  | name is the optional name for the copy; empty -&gt; server derives one (e.g. &#34;&lt;name&gt; (copy)&#34;). |






<a name="cloud-v1-api-CloneTestPresetResponse"></a>

### CloneTestPresetResponse
CloneTestPresetResponse returns the new editable copy.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is the cloned, editable test preset. |






<a name="cloud-v1-api-CloneWorkloadPresetRequest"></a>

### CloneWorkloadPresetRequest
Clone copies a preset (typically a read-only system one) into a new editable
preset owned by the caller. The copy gets a fresh id, is_system = false and
the caller as author.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the source preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to copy. |
| name | [string](#string) |  | name is the optional name for the copy; empty -&gt; server derives one (e.g. &#34;&lt;name&gt; (copy)&#34;). |






<a name="cloud-v1-api-CloneWorkloadPresetResponse"></a>

### CloneWorkloadPresetResponse
CloneWorkloadPresetResponse returns the new editable copy.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-WorkloadPresetRecord) |  | preset is the cloned, editable workload preset. |






<a name="cloud-v1-api-CreateDatabasePresetRequest"></a>

### CreateDatabasePresetRequest
CreateDatabasePresetRequest creates a new database preset in the tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the preset; presets never cross tenants. |
| preset | [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-DatabasePresetRecord) |  | preset is the database preset to create. Server assigns entity.id / tenant_id / timings; values set here are ignored. |






<a name="cloud-v1-api-CreateDatabasePresetResponse"></a>

### CreateDatabasePresetResponse
CreateDatabasePresetResponse returns the created preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-DatabasePresetRecord) |  | preset is the newly created database preset. |






<a name="cloud-v1-api-CreateTestPresetRequest"></a>

### CreateTestPresetRequest
CreateTestPresetRequest creates a new test preset in the tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the preset; presets never cross tenants. |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is the test preset to create. Server assigns entity.id / tenant_id / timings; values set here are ignored. |






<a name="cloud-v1-api-CreateTestPresetResponse"></a>

### CreateTestPresetResponse
CreateTestPresetResponse returns the created preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is the newly created test preset. |






<a name="cloud-v1-api-CreateWorkloadPresetRequest"></a>

### CreateWorkloadPresetRequest
CreateWorkloadPresetRequest creates a new workload preset in the tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the preset; presets never cross tenants. |
| preset | [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-WorkloadPresetRecord) |  | preset is the workload preset to create. Server assigns entity.id / tenant_id / timings; values set here are ignored. |






<a name="cloud-v1-api-CreateWorkloadPresetResponse"></a>

### CreateWorkloadPresetResponse
CreateWorkloadPresetResponse returns the created preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-WorkloadPresetRecord) |  | preset is the newly created workload preset. |






<a name="cloud-v1-api-DeleteDatabasePresetRequest"></a>

### DeleteDatabasePresetRequest
DeleteDatabasePresetRequest removes one database preset by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to remove. |






<a name="cloud-v1-api-DeleteDatabasePresetResponse"></a>

### DeleteDatabasePresetResponse
DeleteDatabasePresetResponse is empty; success is signalled by the absence of
error.






<a name="cloud-v1-api-DeleteTestPresetRequest"></a>

### DeleteTestPresetRequest
DeleteTestPresetRequest removes one test preset by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to remove. |






<a name="cloud-v1-api-DeleteTestPresetResponse"></a>

### DeleteTestPresetResponse
DeleteTestPresetResponse is empty; success is signalled by the absence of
error.






<a name="cloud-v1-api-DeleteWorkloadPresetRequest"></a>

### DeleteWorkloadPresetRequest
DeleteWorkloadPresetRequest removes one workload preset by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to remove. |






<a name="cloud-v1-api-DeleteWorkloadPresetResponse"></a>

### DeleteWorkloadPresetResponse
DeleteWorkloadPresetResponse is empty; success is signalled by the absence of
error.






<a name="cloud-v1-api-GetDatabasePresetRequest"></a>

### GetDatabasePresetRequest
GetDatabasePresetRequest fetches one database preset by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to fetch. |






<a name="cloud-v1-api-GetDatabasePresetResponse"></a>

### GetDatabasePresetResponse
GetDatabasePresetResponse returns the requested database preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-DatabasePresetRecord) |  | preset is the fetched database preset. |






<a name="cloud-v1-api-GetTestPresetRequest"></a>

### GetTestPresetRequest
GetTestPresetRequest fetches one test preset by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to fetch. |






<a name="cloud-v1-api-GetTestPresetResponse"></a>

### GetTestPresetResponse
GetTestPresetResponse returns the requested test preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is the fetched test preset. |






<a name="cloud-v1-api-GetWorkloadPresetRequest"></a>

### GetWorkloadPresetRequest
GetWorkloadPresetRequest fetches one workload preset by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| id | [string](#string) |  | id is the preset to fetch. |






<a name="cloud-v1-api-GetWorkloadPresetResponse"></a>

### GetWorkloadPresetResponse
GetWorkloadPresetResponse returns the requested workload preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-WorkloadPresetRecord) |  | preset is the fetched workload preset. |






<a name="cloud-v1-api-ListDatabasePresetsRequest"></a>

### ListDatabasePresetsRequest
ListDatabasePresetsRequest lists a tenant&#39;s database presets with filters,
sort and paging.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the listing to one tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter is the shared Entity-level filter (search, ids, time windows, soft-delete). |
| tags | [ListDatabasePresetsRequest.TagsEntry](#cloud-v1-api-ListDatabasePresetsRequest-TagsEntry) | repeated | tags is a tag match: every key=value pair must be present on the database (AND). |
| db_kinds | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) | repeated | db_kinds narrows to specific database engines (kind-specific filter). |
| sources | [ListDatabasePresetsRequest.SourceKind](#cloud-v1-api-ListDatabasePresetsRequest-SourceKind) | repeated | sources narrows to specific SourceKinds (kind-specific filter). |
| is_system | [bool](#bool) | optional | is_system filters by the system flag. Unset = all; true = only system; false = only user. |
| sort | [ListDatabasePresetsRequest.Sort](#cloud-v1-api-ListDatabasePresetsRequest-Sort) |  | sort is the ordering (common Entity column or table-specific). |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page is the pagination cursor/size. |






<a name="cloud-v1-api-ListDatabasePresetsRequest-Sort"></a>

### ListDatabasePresetsRequest.Sort
Sort orders by EITHER a common Entity column OR a table-specific column.
desc applies to whichever is chosen.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entity | [cloud.v1.common.EntitySortField](#cloud-v1-common-EntitySortField) |  | entity sorts by a common Entity column. |
| kind | [ListDatabasePresetsRequest.Sort.Kind](#cloud-v1-api-ListDatabasePresetsRequest-Sort-Kind) |  | kind sorts by a table-specific column. |
| desc | [bool](#bool) |  | desc reverses the order when true. |






<a name="cloud-v1-api-ListDatabasePresetsRequest-TagsEntry"></a>

### ListDatabasePresetsRequest.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="cloud-v1-api-ListDatabasePresetsResponse"></a>

### ListDatabasePresetsResponse
ListDatabasePresetsResponse returns one page of database presets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| presets | [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-DatabasePresetRecord) | repeated | presets is this page of database presets. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-ListTestPresetsRequest"></a>

### ListTestPresetsRequest
ListTestPresetsRequest lists a tenant&#39;s test presets with filters, sort and
paging.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the listing to one tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter is the shared Entity-level filter (search, ids, time windows, soft-delete). |
| tags | [ListTestPresetsRequest.TagsEntry](#cloud-v1-api-ListTestPresetsRequest-TagsEntry) | repeated | tags is a tag match over the test tags (AND). |
| db_kinds | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) | repeated | db_kinds narrows to specific database engines (kind-specific filter). |
| stroppy_versions | [string](#string) | repeated | stroppy_versions narrows to specific workload versions (kind-specific filter). |
| is_system | [bool](#bool) | optional | is_system filters by the system flag. Unset = all; true = only system; false = only user. |
| sort | [ListTestPresetsRequest.Sort](#cloud-v1-api-ListTestPresetsRequest-Sort) |  | sort is the ordering (common Entity column or table-specific). |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page is the pagination cursor/size. |






<a name="cloud-v1-api-ListTestPresetsRequest-Sort"></a>

### ListTestPresetsRequest.Sort
Sort orders by EITHER a common Entity column OR a table-specific column.
desc applies to whichever is chosen.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entity | [cloud.v1.common.EntitySortField](#cloud-v1-common-EntitySortField) |  | entity sorts by a common Entity column. |
| kind | [ListTestPresetsRequest.Sort.Kind](#cloud-v1-api-ListTestPresetsRequest-Sort-Kind) |  | kind sorts by a table-specific column. |
| desc | [bool](#bool) |  | desc reverses the order when true. |






<a name="cloud-v1-api-ListTestPresetsRequest-TagsEntry"></a>

### ListTestPresetsRequest.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="cloud-v1-api-ListTestPresetsResponse"></a>

### ListTestPresetsResponse
ListTestPresetsResponse returns one page of test presets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| presets | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) | repeated | presets is this page of test presets. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-ListWorkloadPresetsRequest"></a>

### ListWorkloadPresetsRequest
ListWorkloadPresetsRequest lists a tenant&#39;s workload presets with filters,
sort and paging.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the listing to one tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter is the shared Entity-level filter (search, ids, time windows, soft-delete). |
| tags | [ListWorkloadPresetsRequest.TagsEntry](#cloud-v1-api-ListWorkloadPresetsRequest-TagsEntry) | repeated | tags is a tag match over the workload tags (AND). |
| stroppy_versions | [string](#string) | repeated | stroppy_versions narrows to specific workload versions (kind-specific filter). |
| is_system | [bool](#bool) | optional | is_system filters by the system flag. Unset = all; true = only system; false = only user. |
| sort | [ListWorkloadPresetsRequest.Sort](#cloud-v1-api-ListWorkloadPresetsRequest-Sort) |  | sort is the ordering (common Entity column or table-specific). |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page is the pagination cursor/size. |






<a name="cloud-v1-api-ListWorkloadPresetsRequest-Sort"></a>

### ListWorkloadPresetsRequest.Sort
Sort orders by EITHER a common Entity column OR a table-specific column.
desc applies to whichever is chosen.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entity | [cloud.v1.common.EntitySortField](#cloud-v1-common-EntitySortField) |  | entity sorts by a common Entity column. |
| kind | [ListWorkloadPresetsRequest.Sort.Kind](#cloud-v1-api-ListWorkloadPresetsRequest-Sort-Kind) |  | kind sorts by a table-specific column. |
| desc | [bool](#bool) |  | desc reverses the order when true. |






<a name="cloud-v1-api-ListWorkloadPresetsRequest-TagsEntry"></a>

### ListWorkloadPresetsRequest.TagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="cloud-v1-api-ListWorkloadPresetsResponse"></a>

### ListWorkloadPresetsResponse
ListWorkloadPresetsResponse returns one page of workload presets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| presets | [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-WorkloadPresetRecord) | repeated | presets is this page of workload presets. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-UpdateDatabasePresetRequest"></a>

### UpdateDatabasePresetRequest
UpdateDatabasePresetRequest replaces a database preset wholesale.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| preset | [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-DatabasePresetRecord) |  | preset is the wholesale replacement; preset.entity.id selects the row. |






<a name="cloud-v1-api-UpdateDatabasePresetResponse"></a>

### UpdateDatabasePresetResponse
UpdateDatabasePresetResponse returns the preset after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.DatabasePresetRecord](#cloud-v1-models-DatabasePresetRecord) |  | preset is the updated database preset. |






<a name="cloud-v1-api-UpdateTestPresetRequest"></a>

### UpdateTestPresetRequest
UpdateTestPresetRequest replaces a test preset wholesale.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is the wholesale replacement; preset.entity.id selects the row. |






<a name="cloud-v1-api-UpdateTestPresetResponse"></a>

### UpdateTestPresetResponse
UpdateTestPresetResponse returns the preset after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is the updated test preset. |






<a name="cloud-v1-api-UpdateWorkloadPresetRequest"></a>

### UpdateWorkloadPresetRequest
UpdateWorkloadPresetRequest replaces a workload preset wholesale.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the preset&#39;s tenant. |
| preset | [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-WorkloadPresetRecord) |  | preset is the wholesale replacement; preset.entity.id selects the row. |






<a name="cloud-v1-api-UpdateWorkloadPresetResponse"></a>

### UpdateWorkloadPresetResponse
UpdateWorkloadPresetResponse returns the preset after the edit.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.WorkloadPresetRecord](#cloud-v1-models-WorkloadPresetRecord) |  | preset is the updated workload preset. |





 


<a name="cloud-v1-api-ListDatabasePresetsRequest-Sort-Kind"></a>

### ListDatabasePresetsRequest.Sort.Kind
Kind is the set of table-specific sort columns for database presets.

| Name | Number | Description |
| ---- | ------ | ----------- |
| KIND_UNSPECIFIED | 0 | KIND_UNSPECIFIED is the zero value (no table-specific column). |
| KIND_DB_KIND | 1 | KIND_DB_KIND sorts by domain.Database.Kind. |
| KIND_IS_SYSTEM | 2 | KIND_IS_SYSTEM sorts system presets first/last. |



<a name="cloud-v1-api-ListDatabasePresetsRequest-SourceKind"></a>

### ListDatabasePresetsRequest.SourceKind
SourceKind mirrors domain.Database.source oneof, for filtering.

| Name | Number | Description |
| ---- | ------ | ----------- |
| SOURCE_KIND_UNSPECIFIED | 0 | SOURCE_KIND_UNSPECIFIED is the zero value (no source filter). |
| SOURCE_KIND_PARAMS | 1 | SOURCE_KIND_PARAMS is a self-deploy database (params source). |
| SOURCE_KIND_EXTERNAL | 2 | SOURCE_KIND_EXTERNAL is an external database (dsn source). |
| SOURCE_KIND_PRESET_REF | 3 | SOURCE_KIND_PRESET_REF references another preset (database_preset_id). |



<a name="cloud-v1-api-ListTestPresetsRequest-Sort-Kind"></a>

### ListTestPresetsRequest.Sort.Kind
Kind is the set of table-specific sort columns for test presets.

| Name | Number | Description |
| ---- | ------ | ----------- |
| KIND_UNSPECIFIED | 0 | KIND_UNSPECIFIED is the zero value (no table-specific column). |
| KIND_DB_KIND | 1 | KIND_DB_KIND sorts by the test&#39;s database kind. |
| KIND_STROPPY_VERSION | 2 | KIND_STROPPY_VERSION sorts by the test&#39;s stroppy version. |
| KIND_IS_SYSTEM | 3 | KIND_IS_SYSTEM sorts system presets first/last. |



<a name="cloud-v1-api-ListWorkloadPresetsRequest-Sort-Kind"></a>

### ListWorkloadPresetsRequest.Sort.Kind
Kind is the set of table-specific sort columns for workload presets.

| Name | Number | Description |
| ---- | ------ | ----------- |
| KIND_UNSPECIFIED | 0 | KIND_UNSPECIFIED is the zero value (no table-specific column). |
| KIND_STROPPY_VERSION | 1 | KIND_STROPPY_VERSION sorts by the workload&#39;s stroppy version. |
| KIND_IS_SYSTEM | 2 | KIND_IS_SYSTEM sorts system presets first/last. |


 

 


<a name="cloud-v1-api-DatabasePresetService"></a>

### DatabasePresetService
DatabasePresetService is the tenant-scoped CRUD over database presets.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateDatabasePreset | [CreateDatabasePresetRequest](#cloud-v1-api-CreateDatabasePresetRequest) | [CreateDatabasePresetResponse](#cloud-v1-api-CreateDatabasePresetResponse) | CreateDatabasePreset creates a database preset. Not idempotent: each call mints a new preset. |
| GetDatabasePreset | [GetDatabasePresetRequest](#cloud-v1-api-GetDatabasePresetRequest) | [GetDatabasePresetResponse](#cloud-v1-api-GetDatabasePresetResponse) | GetDatabasePreset fetches one database preset by id. Read-only. |
| ListDatabasePresets | [ListDatabasePresetsRequest](#cloud-v1-api-ListDatabasePresetsRequest) | [ListDatabasePresetsResponse](#cloud-v1-api-ListDatabasePresetsResponse) | ListDatabasePresets lists a tenant&#39;s database presets. Read-only. |
| UpdateDatabasePreset | [UpdateDatabasePresetRequest](#cloud-v1-api-UpdateDatabasePresetRequest) | [UpdateDatabasePresetResponse](#cloud-v1-api-UpdateDatabasePresetResponse) | UpdateDatabasePreset is idempotent: a wholesale field set converges on retry. System presets (is_system) are read-only and rejected — clone instead. |
| DeleteDatabasePreset | [DeleteDatabasePresetRequest](#cloud-v1-api-DeleteDatabasePresetRequest) | [DeleteDatabasePresetResponse](#cloud-v1-api-DeleteDatabasePresetResponse) | DeleteDatabasePreset is idempotent: deleting an absent preset is a no-op. System presets (is_system) are rejected. |
| CloneDatabasePreset | [CloneDatabasePresetRequest](#cloud-v1-api-CloneDatabasePresetRequest) | [CloneDatabasePresetResponse](#cloud-v1-api-CloneDatabasePresetResponse) | CloneDatabasePreset creates a new editable copy. Not idempotent: each call mints a new preset. |


<a name="cloud-v1-api-TestPresetService"></a>

### TestPresetService
TestPresetService is the tenant-scoped CRUD over test presets.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateTestPreset | [CreateTestPresetRequest](#cloud-v1-api-CreateTestPresetRequest) | [CreateTestPresetResponse](#cloud-v1-api-CreateTestPresetResponse) | CreateTestPreset creates a test preset. Not idempotent: each call mints a new preset. |
| GetTestPreset | [GetTestPresetRequest](#cloud-v1-api-GetTestPresetRequest) | [GetTestPresetResponse](#cloud-v1-api-GetTestPresetResponse) | GetTestPreset fetches one test preset by id. Read-only. |
| ListTestPresets | [ListTestPresetsRequest](#cloud-v1-api-ListTestPresetsRequest) | [ListTestPresetsResponse](#cloud-v1-api-ListTestPresetsResponse) | ListTestPresets lists a tenant&#39;s test presets. Read-only. |
| UpdateTestPreset | [UpdateTestPresetRequest](#cloud-v1-api-UpdateTestPresetRequest) | [UpdateTestPresetResponse](#cloud-v1-api-UpdateTestPresetResponse) | UpdateTestPreset is idempotent: a wholesale field set converges on retry. System presets (is_system) are read-only and rejected — clone instead. |
| DeleteTestPreset | [DeleteTestPresetRequest](#cloud-v1-api-DeleteTestPresetRequest) | [DeleteTestPresetResponse](#cloud-v1-api-DeleteTestPresetResponse) | DeleteTestPreset is idempotent: deleting an absent preset is a no-op. System presets (is_system) are rejected. |
| CloneTestPreset | [CloneTestPresetRequest](#cloud-v1-api-CloneTestPresetRequest) | [CloneTestPresetResponse](#cloud-v1-api-CloneTestPresetResponse) | CloneTestPreset creates a new editable copy. Not idempotent: each call mints a new preset. |


<a name="cloud-v1-api-WorkloadPresetService"></a>

### WorkloadPresetService
WorkloadPresetService is the tenant-scoped CRUD over workload presets.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateWorkloadPreset | [CreateWorkloadPresetRequest](#cloud-v1-api-CreateWorkloadPresetRequest) | [CreateWorkloadPresetResponse](#cloud-v1-api-CreateWorkloadPresetResponse) | CreateWorkloadPreset creates a workload preset. Not idempotent: each call mints a new preset. |
| GetWorkloadPreset | [GetWorkloadPresetRequest](#cloud-v1-api-GetWorkloadPresetRequest) | [GetWorkloadPresetResponse](#cloud-v1-api-GetWorkloadPresetResponse) | GetWorkloadPreset fetches one workload preset by id. Read-only. |
| ListWorkloadPresets | [ListWorkloadPresetsRequest](#cloud-v1-api-ListWorkloadPresetsRequest) | [ListWorkloadPresetsResponse](#cloud-v1-api-ListWorkloadPresetsResponse) | ListWorkloadPresets lists a tenant&#39;s workload presets. Read-only. |
| UpdateWorkloadPreset | [UpdateWorkloadPresetRequest](#cloud-v1-api-UpdateWorkloadPresetRequest) | [UpdateWorkloadPresetResponse](#cloud-v1-api-UpdateWorkloadPresetResponse) | UpdateWorkloadPreset is idempotent: a wholesale field set converges on retry. System presets (is_system) are read-only and rejected — clone instead. |
| DeleteWorkloadPreset | [DeleteWorkloadPresetRequest](#cloud-v1-api-DeleteWorkloadPresetRequest) | [DeleteWorkloadPresetResponse](#cloud-v1-api-DeleteWorkloadPresetResponse) | DeleteWorkloadPreset is idempotent: deleting an absent preset is a no-op. System presets (is_system) are rejected. |
| CloneWorkloadPreset | [CloneWorkloadPresetRequest](#cloud-v1-api-CloneWorkloadPresetRequest) | [CloneWorkloadPresetResponse](#cloud-v1-api-CloneWorkloadPresetResponse) | CloneWorkloadPreset creates a new editable copy. Not idempotent: each call mints a new preset. |

 



<a name="cloud_v1_api_public_rating-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/public_rating.proto



<a name="cloud-v1-api-GetPublicRatingRequest"></a>

### GetPublicRatingRequest
GetPublicRatingRequest selects and pages the public leaderboard.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filter | [RatingFilter](#cloud-v1-api-RatingFilter) |  | filter reuses the authenticated filter shape (metric_key &#43; facets). |
| limit | [uint32](#uint32) |  | limit caps returned entries (&lt;= 500); 0 -&gt; server default. |
| page_token | [string](#string) |  | page_token is the opaque cursor from a previous response. |






<a name="cloud-v1-api-GetPublicRatingResponse"></a>

### GetPublicRatingResponse
GetPublicRatingResponse returns one page of the public leaderboard.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entries | [PublicRatingEntry](#cloud-v1-api-PublicRatingEntry) | repeated | entries are the ranked benchmarks for this page (sensitive fields stripped). |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-PublicRatingEntry"></a>

### PublicRatingEntry
PublicRatingEntry is one ranked benchmark, sensitive fields stripped.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rank | [uint32](#uint32) |  | rank is the 1-based position on the leaderboard. |
| metric_value | [double](#double) |  | metric_value is the ranked metric&#39;s value for this entry. |
| metric_unit | [string](#string) |  | metric_unit is the unit the metric_value is expressed in. |
| db_kind | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) |  | db_kind is the database engine the benchmark ran against. |
| workload_name | [string](#string) |  | workload_name is the workload the benchmark executed. |
| stroppy_version | [string](#string) |  | stroppy_version is the stroppy engine version used. |
| provider | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) |  | provider is the deployment/cloud provider the benchmark ran on. |
| topology_label | [string](#string) |  | topology_label is a human-readable summary of the cluster topology. |
| node_count | [uint32](#uint32) |  | node_count is the number of nodes in the topology. |





 

 

 


<a name="cloud-v1-api-PublicRatingService"></a>

### PublicRatingService
PublicRatingService serves the public, unauthenticated benchmark leaderboard.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetPublicRating | [GetPublicRatingRequest](#cloud-v1-api-GetPublicRatingRequest) | [GetPublicRatingResponse](#cloud-v1-api-GetPublicRatingResponse) | GetPublicRating resolves the public leaderboard. PUBLIC: no bearer token. |

 



<a name="cloud_v1_api_public_share-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/public_share.proto



<a name="cloud-v1-api-GetSharedRunRequest"></a>

### GetSharedRunRequest
GetSharedRunRequest resolves one share by its public token.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| token | [string](#string) |  | token is the unguessable token from the share URL. |






<a name="cloud-v1-api-GetSharedRunResponse"></a>

### GetSharedRunResponse
GetSharedRunResponse returns the limited public snapshot for the token.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| snapshot | [cloud.v1.models.ShareRecord.Snapshot](#cloud-v1-models-ShareRecord-Snapshot) |  | snapshot is the limited public snapshot (test or suite run) &#43; when it was captured. |





 

 

 


<a name="cloud-v1-api-PublicShareService"></a>

### PublicShareService
PublicShareService is the public, unauthenticated resolve side of run shares.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetSharedRun | [GetSharedRunRequest](#cloud-v1-api-GetSharedRunRequest) | [GetSharedRunResponse](#cloud-v1-api-GetSharedRunResponse) | GetSharedRun resolves a share token to its limited snapshot. PUBLIC: no bearer token. Returns gone if revoked/expired. |

 



<a name="cloud_v1_api_rating-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/rating.proto



<a name="cloud-v1-api-GetSystemRatingRequest"></a>

### GetSystemRatingRequest
GetSystemRatingRequest selects and pages the cross-system private board.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| filter | [RatingFilter](#cloud-v1-api-RatingFilter) |  | filter selects and ranks the benchmark runs. |
| limit | [uint32](#uint32) |  | limit caps returned entries (&lt;= 500); 0 -&gt; server default. |
| page_token | [string](#string) |  | page_token is the opaque cursor from a previous response. |






<a name="cloud-v1-api-GetSystemRatingResponse"></a>

### GetSystemRatingResponse
GetSystemRatingResponse returns one page of the system-wide board.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entries | [RatingEntry](#cloud-v1-api-RatingEntry) | repeated | entries are the ranked benchmarks for this page. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-GetTenantRatingRequest"></a>

### GetTenantRatingRequest
GetTenantRatingRequest selects and pages one tenant&#39;s board.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the board to one tenant. |
| filter | [RatingFilter](#cloud-v1-api-RatingFilter) |  | filter selects and ranks the benchmark runs. |
| limit | [uint32](#uint32) |  | limit caps returned entries (&lt;= 500); 0 -&gt; server default. |
| page_token | [string](#string) |  | page_token is the opaque cursor from a previous response. |






<a name="cloud-v1-api-GetTenantRatingResponse"></a>

### GetTenantRatingResponse
GetTenantRatingResponse returns one page of the tenant&#39;s board.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entries | [RatingEntry](#cloud-v1-api-RatingEntry) | repeated | entries are the ranked benchmarks for this page. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-RatingEntry"></a>

### RatingEntry
RatingEntry is one ranked benchmark for the authenticated boards. Includes
identifying info (run_id, author, tenant) the public board omits.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| rank | [uint32](#uint32) |  | rank is the 1-based position on the leaderboard. |
| metric_value | [double](#double) |  | metric_value is the ranked metric&#39;s value for this entry. |
| metric_unit | [string](#string) |  | metric_unit is the unit the metric_value is expressed in. |
| db_kind | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) |  | db_kind is the database engine the benchmark ran against. |
| workload_name | [string](#string) |  | workload_name is the workload the benchmark executed. |
| stroppy_version | [string](#string) |  | stroppy_version is the stroppy engine version used. |
| provider | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) |  | provider is the deployment/cloud provider the benchmark ran on. |
| topology_label | [string](#string) |  | topology_label is a human-readable summary of the cluster topology. |
| node_count | [uint32](#uint32) |  | node_count is the number of nodes in the topology. |
| run_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | run_at is when the benchmark run started. |
| run_id | [string](#string) |  | run_id identifies the underlying test run (authenticated scopes only). |
| author_name | [string](#string) |  | author_name is who created the run (authenticated scopes only). |
| tenant_name | [string](#string) |  | tenant_name is the owning tenant; set only on the system-wide board. |






<a name="cloud-v1-api-RatingFilter"></a>

### RatingFilter
RatingFilter selects and ranks the benchmark runs. Ranking is by one metric
(metric_key); direction is the metric&#39;s own higher_is_better.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metric_key | [string](#string) |  | metric_key is the metric to rank by (a MetricSummary.key). Required. |
| db_kinds | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) | repeated | db_kinds narrows to specific database engines (AND; empty = not applied). |
| stroppy_versions | [string](#string) | repeated | stroppy_versions narrows to specific engine versions (AND; empty = not applied). |
| providers | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) | repeated | providers narrows to specific deployment providers (AND; empty = not applied). |
| started_after | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_after keeps only runs that started at/after this time. |
| started_before | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_before keeps only runs that started at/before this time. |





 

 

 


<a name="cloud-v1-api-RatingService"></a>

### RatingService
RatingService serves the authenticated benchmark leaderboards (system &#43; tenant
scopes).

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetSystemRating | [GetSystemRatingRequest](#cloud-v1-api-GetSystemRatingRequest) | [GetSystemRatingResponse](#cloud-v1-api-GetSystemRatingResponse) | GetSystemRating: cross-system private board over in_global_rating runs. Any authenticated account (no specific permission). |
| GetTenantRating | [GetTenantRatingRequest](#cloud-v1-api-GetTenantRatingRequest) | [GetTenantRatingResponse](#cloud-v1-api-GetTenantRatingResponse) | GetTenantRating: this tenant&#39;s board over in_tenant_rating runs. |

 



<a name="cloud_v1_api_share-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/share.proto



<a name="cloud-v1-api-CreateShareRequest"></a>

### CreateShareRequest
CreateShareRequest mints a new share (token &#43; first snapshot) for a run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the share; shares are tenant-scoped. |
| target | [cloud.v1.models.ShareRecord.Target](#cloud-v1-models-ShareRecord-Target) |  | target is what to share (test run / suite run &#43; id). |
| ttl | [google.protobuf.Duration](#google-protobuf-Duration) |  | ttl is the share lifetime. 0 / unset -&gt; server default (1 week). Explicit 0 to mean &#34;never&#34; is allowed but the client SHOULD warn the user: it is insecure and keeps the background refresh running forever. |






<a name="cloud-v1-api-CreateShareResponse"></a>

### CreateShareResponse
CreateShareResponse returns the created share record (with its token).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| share | [cloud.v1.models.ShareRecord](#cloud-v1-models-ShareRecord) |  | share is the newly created share record. |






<a name="cloud-v1-api-DeleteShareRequest"></a>

### DeleteShareRequest
DeleteShareRequest removes one share by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the share&#39;s tenant. |
| id | [string](#string) |  | id is the share to remove. |






<a name="cloud-v1-api-DeleteShareResponse"></a>

### DeleteShareResponse
DeleteShareResponse is empty; success is signalled by the absence of error.






<a name="cloud-v1-api-GetShareRequest"></a>

### GetShareRequest
GetShareRequest fetches one share by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the share&#39;s tenant. |
| id | [string](#string) |  | id is the share to fetch. |






<a name="cloud-v1-api-GetShareResponse"></a>

### GetShareResponse
GetShareResponse returns the requested share.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| share | [cloud.v1.models.ShareRecord](#cloud-v1-models-ShareRecord) |  | share is the fetched share record. |






<a name="cloud-v1-api-ListSharesRequest"></a>

### ListSharesRequest
ListSharesRequest lists a tenant&#39;s shares with filtering, sort and paging.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the listing to one tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter is the shared Entity-level filter (search, ids, time windows). |
| sort | [cloud.v1.common.EntitySort](#cloud-v1-common-EntitySort) |  | sort is the ordering over the common Entity columns. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page is the pagination cursor/size. |
| target_id | [string](#string) |  | target_id narrows to shares of one run; empty = any. |






<a name="cloud-v1-api-ListSharesResponse"></a>

### ListSharesResponse
ListSharesResponse returns one page of shares.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| shares | [cloud.v1.models.ShareRecord](#cloud-v1-models-ShareRecord) | repeated | shares is this page of share records. |
| next_page_token | [string](#string) |  | next_page_token is empty when there are no more rows. |






<a name="cloud-v1-api-RevokeShareRequest"></a>

### RevokeShareRequest
RevokeShare disables a share (the public endpoint returns gone) without
deleting the record. Idempotent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the share&#39;s tenant. |
| id | [string](#string) |  | id is the share to disable. |






<a name="cloud-v1-api-RevokeShareResponse"></a>

### RevokeShareResponse
RevokeShareResponse returns the share after it was disabled.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| share | [cloud.v1.models.ShareRecord](#cloud-v1-models-ShareRecord) |  | share is the now-revoked share record. |






<a name="cloud-v1-api-SetShareExpiryRequest"></a>

### SetShareExpiryRequest
SetShareExpiry changes the lifetime (extend / shorten). Same ttl semantics as
create (0/unset = default, explicit 0 = never with a warning).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the share&#39;s tenant. |
| id | [string](#string) |  | id is the share whose lifetime to change. |
| ttl | [google.protobuf.Duration](#google-protobuf-Duration) |  | ttl is the new lifetime; same semantics as create (0/unset = default, explicit 0 = never with a warning). |






<a name="cloud-v1-api-SetShareExpiryResponse"></a>

### SetShareExpiryResponse
SetShareExpiryResponse returns the share after its lifetime was changed.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| share | [cloud.v1.models.ShareRecord](#cloud-v1-models-ShareRecord) |  | share is the share record with the updated expiry. |





 

 

 


<a name="cloud-v1-api-ShareService"></a>

### ShareService
ShareService is the authenticated management side of run shares.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateShare | [CreateShareRequest](#cloud-v1-api-CreateShareRequest) | [CreateShareResponse](#cloud-v1-api-CreateShareResponse) | CreateShare is not idempotent: each call mints a new token &#43; snapshot. |
| GetShare | [GetShareRequest](#cloud-v1-api-GetShareRequest) | [GetShareResponse](#cloud-v1-api-GetShareResponse) | GetShare fetches one share by id. Read-only. |
| ListShares | [ListSharesRequest](#cloud-v1-api-ListSharesRequest) | [ListSharesResponse](#cloud-v1-api-ListSharesResponse) | ListShares lists a tenant&#39;s shares. Read-only. |
| RevokeShare | [RevokeShareRequest](#cloud-v1-api-RevokeShareRequest) | [RevokeShareResponse](#cloud-v1-api-RevokeShareResponse) | RevokeShare is idempotent: revoking an already-revoked share is a no-op. |
| SetShareExpiry | [SetShareExpiryRequest](#cloud-v1-api-SetShareExpiryRequest) | [SetShareExpiryResponse](#cloud-v1-api-SetShareExpiryResponse) | SetShareExpiry is idempotent: setting the same lifetime converges. |
| DeleteShare | [DeleteShareRequest](#cloud-v1-api-DeleteShareRequest) | [DeleteShareResponse](#cloud-v1-api-DeleteShareResponse) | DeleteShare is idempotent: deleting an absent share is a no-op. |

 



<a name="cloud_v1_api_suite-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/suite.proto



<a name="cloud-v1-api-CloneSuiteRequest"></a>

### CloneSuiteRequest
CloneSuite copies a suite definition into a new editable one owned by the
caller (fresh id, caller as author).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the source suite definition to clone. |
| name | [string](#string) |  | name is the optional name for the clone; empty -&gt; server derives one. |






<a name="cloud-v1-api-CloneSuiteResponse"></a>

### CloneSuiteResponse
CloneSuiteResponse returns the freshly cloned suite definition.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) |  | suite is the new definition owned by the caller. |






<a name="cloud-v1-api-CreateSuiteRequest"></a>

### CreateSuiteRequest
CreateSuiteRequest creates a new suite definition under a tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| suite | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) |  | suite is the definition to persist. Server assigns entity.id / tenant_id / timings. |






<a name="cloud-v1-api-CreateSuiteResponse"></a>

### CreateSuiteResponse
CreateSuiteResponse returns the newly created suite definition.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) |  | suite is the persisted definition with server-assigned fields populated. |






<a name="cloud-v1-api-DeleteSuiteRequest"></a>

### DeleteSuiteRequest
DeleteSuiteRequest deletes a suite definition by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the suite definition identifier to delete. |






<a name="cloud-v1-api-DeleteSuiteResponse"></a>

### DeleteSuiteResponse
DeleteSuiteResponse is empty; deletion success is signalled by a non-error reply.






<a name="cloud-v1-api-GetSuiteRequest"></a>

### GetSuiteRequest
GetSuiteRequest fetches a single suite definition by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the suite definition identifier to fetch. |






<a name="cloud-v1-api-GetSuiteResponse"></a>

### GetSuiteResponse
GetSuiteResponse returns the requested suite definition.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) |  | suite is the requested definition. |






<a name="cloud-v1-api-ListSuitesRequest"></a>

### ListSuitesRequest
ListSuitesRequest lists suite definitions with filtering, faceting, sorting
and pagination.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter holds the shared Entity-level filters (search, ids, time windows). |
| providers | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) | repeated | providers restricts the listing to suites targeting these providers (facet filter). |
| schedule_enabled | [bool](#bool) | optional | schedule_enabled facets by schedule state. Unset = any; true = only scheduled&#43;enabled; false = only paused/none. |
| sort | [ListSuitesRequest.Sort](#cloud-v1-api-ListSuitesRequest-Sort) |  | sort selects the result ordering. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page carries pagination (page size &#43; token). |






<a name="cloud-v1-api-ListSuitesRequest-Sort"></a>

### ListSuitesRequest.Sort
Sort selects the ordering column for the listing.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entity | [cloud.v1.common.EntitySortField](#cloud-v1-common-EntitySortField) |  | entity orders by a shared Entity column (name, created_at, ...). |
| kind | [ListSuitesRequest.Sort.Kind](#cloud-v1-api-ListSuitesRequest-Sort-Kind) |  | kind orders by a suite-specific column. |
| desc | [bool](#bool) |  | desc reverses the order (descending) when true. |






<a name="cloud-v1-api-ListSuitesResponse"></a>

### ListSuitesResponse
ListSuitesResponse returns a page of suite definitions.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suites | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) | repeated | suites is the matching page of definitions. |
| next_page_token | [string](#string) |  | next_page_token fetches the following page; empty when at the end. |






<a name="cloud-v1-api-SetSuiteScheduleRequest"></a>

### SetSuiteScheduleRequest
SetSuiteSchedule sets/replaces a suite&#39;s cron schedule (and its enabled flag)
without sending the whole definition — handy for pausing/resuming auto-runs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the suite definition whose schedule is being set. |
| schedule | [cloud.v1.domain.Schedule](#cloud-v1-domain-Schedule) |  | schedule is the cron schedule (and enabled flag) to apply. |






<a name="cloud-v1-api-SetSuiteScheduleResponse"></a>

### SetSuiteScheduleResponse
SetSuiteScheduleResponse returns the suite definition with the new schedule.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) |  | suite is the stored definition after the schedule change. |






<a name="cloud-v1-api-StartSuiteRequest"></a>

### StartSuiteRequest
StartSuite expands a suite into a SuiteRunRecord (one TestRunRecord per
compatible cell) and launches SuiteWorkflow. Provide a stored `suite_id`
(re-run) or a fully-baked `suite` directly (CLI).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| suite_id | [string](#string) |  | suite_id re-runs a stored suite definition by id. |
| suite | [cloud.v1.domain.Suite](#cloud-v1-domain-Suite) |  | suite is a fully-baked suite supplied directly (CLI path). |
| max_parallel | [uint32](#uint32) |  | max_parallel caps concurrent child TestWorkflows. 0 = unlimited. |
| in_tenant_rating | [bool](#bool) | optional | in_tenant_rating sets tenant-rating membership for all child runs; unset -&gt; suite defaults, then platform defaults (tenant true). |
| in_global_rating | [bool](#bool) | optional | in_global_rating sets global-rating membership for all child runs; unset -&gt; suite defaults, then platform defaults (global false). |






<a name="cloud-v1-api-StartSuiteResponse"></a>

### StartSuiteResponse
StartSuiteResponse returns the newly created suite run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite_run | [cloud.v1.models.SuiteRunRecord](#cloud-v1-models-SuiteRunRecord) |  | suite_run is the launched suite execution record. |






<a name="cloud-v1-api-UpdateSuiteRequest"></a>

### UpdateSuiteRequest
UpdateSuiteRequest wholesale-replaces an existing suite definition.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| suite | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) |  | suite is the full replacement; suite.entity.id selects the row. |






<a name="cloud-v1-api-UpdateSuiteResponse"></a>

### UpdateSuiteResponse
UpdateSuiteResponse returns the updated suite definition.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite | [cloud.v1.models.SuiteRecord](#cloud-v1-models-SuiteRecord) |  | suite is the stored definition after the update. |





 


<a name="cloud-v1-api-ListSuitesRequest-Sort-Kind"></a>

### ListSuitesRequest.Sort.Kind
Kind is a suite-specific sortable column (alternative to a common
Entity column).

| Name | Number | Description |
| ---- | ------ | ----------- |
| KIND_UNSPECIFIED | 0 | KIND_UNSPECIFIED leaves the suite-specific ordering unset. |
| KIND_PROVIDER | 1 | KIND_PROVIDER orders by deployment provider. |
| KIND_SCHEDULE_ENABLED | 2 | KIND_SCHEDULE_ENABLED orders by whether the cron schedule is enabled. |
| KIND_NEXT_RUN_AT | 3 | KIND_NEXT_RUN_AT orders by the next planned auto-run time. |
| KIND_LAST_RUN_AT | 4 | KIND_LAST_RUN_AT orders by the most recent run time. |
| KIND_RUN_COUNT | 5 | KIND_RUN_COUNT orders by the total number of runs. |


 

 


<a name="cloud-v1-api-SuiteService"></a>

### SuiteService
SuiteService is the tenant-scoped CRUD &#43; lifecycle API for suite definitions.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateSuite | [CreateSuiteRequest](#cloud-v1-api-CreateSuiteRequest) | [CreateSuiteResponse](#cloud-v1-api-CreateSuiteResponse) | CreateSuite persists a new suite definition. Not idempotent. |
| GetSuite | [GetSuiteRequest](#cloud-v1-api-GetSuiteRequest) | [GetSuiteResponse](#cloud-v1-api-GetSuiteResponse) | GetSuite fetches a single suite definition by id. Read-only. |
| ListSuites | [ListSuitesRequest](#cloud-v1-api-ListSuitesRequest) | [ListSuitesResponse](#cloud-v1-api-ListSuitesResponse) | ListSuites lists suite definitions with filtering and pagination. Read-only. |
| UpdateSuite | [UpdateSuiteRequest](#cloud-v1-api-UpdateSuiteRequest) | [UpdateSuiteResponse](#cloud-v1-api-UpdateSuiteResponse) | UpdateSuite is idempotent: a wholesale field set converges on retry. |
| DeleteSuite | [DeleteSuiteRequest](#cloud-v1-api-DeleteSuiteRequest) | [DeleteSuiteResponse](#cloud-v1-api-DeleteSuiteResponse) | DeleteSuite is idempotent: deleting an absent suite is a no-op. |
| CloneSuite | [CloneSuiteRequest](#cloud-v1-api-CloneSuiteRequest) | [CloneSuiteResponse](#cloud-v1-api-CloneSuiteResponse) | CloneSuite mints a new definition. Not idempotent. |
| SetSuiteSchedule | [SetSuiteScheduleRequest](#cloud-v1-api-SetSuiteScheduleRequest) | [SetSuiteScheduleResponse](#cloud-v1-api-SetSuiteScheduleResponse) | SetSuiteSchedule is idempotent: setting the same schedule converges. |
| StartSuite | [StartSuiteRequest](#cloud-v1-api-StartSuiteRequest) | [StartSuiteResponse](#cloud-v1-api-StartSuiteResponse) | StartSuite mints a SuiteRunRecord. Not idempotent: each call launches a run. |

 



<a name="cloud_v1_api_suite_run-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/suite_run.proto



<a name="cloud-v1-api-CancelSuiteRunRequest"></a>

### CancelSuiteRunRequest
CancelSuiteRun requests cancellation of the suite run and its children.
Idempotent: cancelling a finished/cancelled suite run is a no-op.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the suite run identifier to cancel. |






<a name="cloud-v1-api-CancelSuiteRunResponse"></a>

### CancelSuiteRunResponse
CancelSuiteRunResponse returns the suite run after the cancellation request.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite_run | [cloud.v1.models.SuiteRunRecord](#cloud-v1-models-SuiteRunRecord) |  | suite_run is the execution record reflecting the cancellation. |






<a name="cloud-v1-api-DeleteSuiteRunRequest"></a>

### DeleteSuiteRunRequest
DeleteSuiteRunRequest deletes a suite run by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the suite run identifier to delete. |






<a name="cloud-v1-api-DeleteSuiteRunResponse"></a>

### DeleteSuiteRunResponse
DeleteSuiteRunResponse is empty; deletion success is signalled by a non-error
reply.






<a name="cloud-v1-api-GetSuiteRunRequest"></a>

### GetSuiteRunRequest
GetSuiteRunRequest fetches a single suite run by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the suite run identifier to fetch. |






<a name="cloud-v1-api-GetSuiteRunResponse"></a>

### GetSuiteRunResponse
GetSuiteRunResponse returns the requested suite run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite_run | [cloud.v1.models.SuiteRunRecord](#cloud-v1-models-SuiteRunRecord) |  | suite_run is the requested execution record. |






<a name="cloud-v1-api-ListSuiteRunsRequest"></a>

### ListSuiteRunsRequest
ListSuiteRunsRequest lists suite runs with filtering, faceting, sorting and
pagination.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter holds the shared Entity-level filters (search, ids, time windows). |
| statuses | [cloud.v1.common.Status](#cloud-v1-common-Status) | repeated | statuses facets by lifecycle status (all AND; empty/unset = not applied). |
| providers | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) | repeated | providers facets by deployment provider (empty = any). |
| db_kinds | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) | repeated | db_kinds facets by distinct database kinds exercised (matches Summary.db_kinds). |
| suite_id | [string](#string) |  | suite_id filters to runs of a single suite definition; empty = any. |
| triggers | [cloud.v1.common.Trigger](#cloud-v1-common-Trigger) | repeated | triggers filters by how the run was triggered (manual / cron / api); empty = any. |
| progress_min | [uint32](#uint32) | optional | progress_min is the lower bound of the aggregate progress window, 0..100. |
| progress_max | [uint32](#uint32) | optional | progress_max is the upper bound of the aggregate progress window, 0..100. |
| duration_min | [google.protobuf.Duration](#google-protobuf-Duration) |  | duration_min is the lower bound of the duration window. |
| duration_max | [google.protobuf.Duration](#google-protobuf-Duration) |  | duration_max is the upper bound of the duration window. |
| started_after | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_after keeps runs started at or after this time. |
| started_before | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_before keeps runs started at or before this time. |
| finished_after | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | finished_after keeps runs finished at or after this time. |
| finished_before | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | finished_before keeps runs finished at or before this time. |
| sort | [ListSuiteRunsRequest.Sort](#cloud-v1-api-ListSuiteRunsRequest-Sort) |  | sort selects the result ordering. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page carries pagination (page size &#43; token). |






<a name="cloud-v1-api-ListSuiteRunsRequest-Sort"></a>

### ListSuiteRunsRequest.Sort
Sort orders by a common Entity column OR a denormalized Summary column.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entity | [cloud.v1.common.EntitySortField](#cloud-v1-common-EntitySortField) |  | entity orders by a shared Entity column (name, created_at, ...). |
| kind | [ListSuiteRunsRequest.Sort.Kind](#cloud-v1-api-ListSuiteRunsRequest-Sort-Kind) |  | kind orders by a suite-run-specific column. |
| desc | [bool](#bool) |  | desc reverses the order (descending) when true. |






<a name="cloud-v1-api-ListSuiteRunsResponse"></a>

### ListSuiteRunsResponse
ListSuiteRunsResponse returns a page of suite runs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite_runs | [cloud.v1.models.SuiteRunRecord](#cloud-v1-models-SuiteRunRecord) | repeated | suite_runs is the matching page of execution records. |
| next_page_token | [string](#string) |  | next_page_token fetches the following page; empty when at the end. |





 


<a name="cloud-v1-api-ListSuiteRunsRequest-Sort-Kind"></a>

### ListSuiteRunsRequest.Sort.Kind
Kind is a suite-run-specific sortable column (alternative to a common
Entity column).

| Name | Number | Description |
| ---- | ------ | ----------- |
| KIND_UNSPECIFIED | 0 | KIND_UNSPECIFIED leaves the suite-run-specific ordering unset. |
| KIND_STATUS | 1 | KIND_STATUS orders by lifecycle status. |
| KIND_PROVIDER | 2 | KIND_PROVIDER orders by deployment provider. |
| KIND_PROGRESS | 3 | KIND_PROGRESS orders by aggregate progress. |
| KIND_DURATION | 4 | KIND_DURATION orders by run duration. |
| KIND_STARTED_AT | 5 | KIND_STARTED_AT orders by start time. |
| KIND_FINISHED_AT | 6 | KIND_FINISHED_AT orders by finish time. |
| KIND_TOTAL | 7 | KIND_TOTAL orders by total child runs. |


 

 


<a name="cloud-v1-api-SuiteRunService"></a>

### SuiteRunService
SuiteRunService is the tenant-scoped read/cancel/delete API for suite runs.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetSuiteRun | [GetSuiteRunRequest](#cloud-v1-api-GetSuiteRunRequest) | [GetSuiteRunResponse](#cloud-v1-api-GetSuiteRunResponse) | GetSuiteRun fetches a single suite run by id. Read-only. |
| ListSuiteRuns | [ListSuiteRunsRequest](#cloud-v1-api-ListSuiteRunsRequest) | [ListSuiteRunsResponse](#cloud-v1-api-ListSuiteRunsResponse) | ListSuiteRuns lists suite runs with filtering and pagination. Read-only. |
| CancelSuiteRun | [CancelSuiteRunRequest](#cloud-v1-api-CancelSuiteRunRequest) | [CancelSuiteRunResponse](#cloud-v1-api-CancelSuiteRunResponse) | CancelSuiteRun is idempotent: cancelling a finished/cancelled run is a no-op. |
| DeleteSuiteRun | [DeleteSuiteRunRequest](#cloud-v1-api-DeleteSuiteRunRequest) | [DeleteSuiteRunResponse](#cloud-v1-api-DeleteSuiteRunResponse) | DeleteSuiteRun is idempotent: deleting an absent suite run is a no-op. |

 



<a name="cloud_v1_api_suite_wizard-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/suite_wizard.proto



<a name="cloud-v1-api-DeleteSuiteWizardDraftRequest"></a>

### DeleteSuiteWizardDraftRequest
DeleteSuiteWizardDraftRequest deletes a wizard draft by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft identifier to delete. |






<a name="cloud-v1-api-DeleteSuiteWizardDraftResponse"></a>

### DeleteSuiteWizardDraftResponse
DeleteSuiteWizardDraftResponse is empty; deletion success is signalled by a
non-error reply.






<a name="cloud-v1-api-FinishSuiteWizardRequest"></a>

### FinishSuiteWizardRequest
FinishSuiteWizard bakes the draft into a domain.SuiteRun: every compatible cell
becomes a fully baked TestRun (preset params &#43; the suite&#39;s provider settings &#43;
derived topology). Rejected unless draft.ready.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft to bake. |






<a name="cloud-v1-api-FinishSuiteWizardResponse"></a>

### FinishSuiteWizardResponse
FinishSuiteWizardResponse returns the baked suite run spec.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite_run | [cloud.v1.domain.SuiteRun](#cloud-v1-domain-SuiteRun) |  | suite_run is the baked domain.SuiteRun (every compatible cell as a baked TestRun). |






<a name="cloud-v1-api-GetSuiteWizardDraftRequest"></a>

### GetSuiteWizardDraftRequest
GetSuiteWizardDraftRequest fetches a single wizard draft by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft identifier to fetch. |






<a name="cloud-v1-api-GetSuiteWizardDraftResponse"></a>

### GetSuiteWizardDraftResponse
GetSuiteWizardDraftResponse returns the requested wizard draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| draft | [cloud.v1.models.SuiteWizardDraftRecord](#cloud-v1-models-SuiteWizardDraftRecord) |  | draft is the requested wizard draft. |






<a name="cloud-v1-api-ListSuiteWizardDraftsRequest"></a>

### ListSuiteWizardDraftsRequest
ListSuiteWizardDraftsRequest lists wizard drafts with filtering, sorting and
pagination.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter holds the shared Entity-level filters (author, time windows, ...). |
| sort | [cloud.v1.common.EntitySort](#cloud-v1-common-EntitySort) |  | sort selects the result ordering. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page carries pagination (page size &#43; token). |






<a name="cloud-v1-api-ListSuiteWizardDraftsResponse"></a>

### ListSuiteWizardDraftsResponse
ListSuiteWizardDraftsResponse returns a page of wizard drafts.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| drafts | [cloud.v1.models.SuiteWizardDraftRecord](#cloud-v1-models-SuiteWizardDraftRecord) | repeated | drafts is the matching page of wizard drafts. |
| next_page_token | [string](#string) |  | next_page_token fetches the following page; empty when at the end. |






<a name="cloud-v1-api-PatchSuiteWizardRequest"></a>

### PatchSuiteWizardRequest
PatchSuiteWizard submits the edited form. The server validates it, prunes the
matrix to compatible cells, expands the preview, recomputes readiness and
returns the full new draft (form may carry a re-emitted schema when the
selected presets changed the active matrix).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft being edited. |
| form | [schemapb.Filled](#schemapb-Filled) |  | form carries the edited form values (Filled = values &#43; schema ref). |






<a name="cloud-v1-api-PatchSuiteWizardResponse"></a>

### PatchSuiteWizardResponse
PatchSuiteWizardResponse returns the recomputed wizard draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| draft | [cloud.v1.models.SuiteWizardDraftRecord](#cloud-v1-models-SuiteWizardDraftRecord) |  | draft is the full new draft after validation/expansion/readiness recompute. |






<a name="cloud-v1-api-StartSuiteWizardRequest"></a>

### StartSuiteWizardRequest
StartSuiteWizardRequest opens a new suite wizard draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| name | [string](#string) |  | name is the optional human label for the draft. |
| suite_id | [string](#string) |  | suite_id optionally seeds the draft from an existing suite definition. |






<a name="cloud-v1-api-StartSuiteWizardResponse"></a>

### StartSuiteWizardResponse
StartSuiteWizardResponse returns the freshly created draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| draft | [cloud.v1.models.SuiteWizardDraftRecord](#cloud-v1-models-SuiteWizardDraftRecord) |  | draft is the new wizard draft (carrying the initial form schema). |





 

 

 


<a name="cloud-v1-api-SuiteWizardService"></a>

### SuiteWizardService
SuiteWizardService drives the suite wizard: start, fetch/list, patch-loop and
finish into a baked SuiteRun.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| StartSuiteWizard | [StartSuiteWizardRequest](#cloud-v1-api-StartSuiteWizardRequest) | [StartSuiteWizardResponse](#cloud-v1-api-StartSuiteWizardResponse) | StartSuiteWizard opens a new draft. Not idempotent. |
| GetSuiteWizardDraft | [GetSuiteWizardDraftRequest](#cloud-v1-api-GetSuiteWizardDraftRequest) | [GetSuiteWizardDraftResponse](#cloud-v1-api-GetSuiteWizardDraftResponse) | GetSuiteWizardDraft fetches a single draft by id. Read-only. |
| ListSuiteWizardDrafts | [ListSuiteWizardDraftsRequest](#cloud-v1-api-ListSuiteWizardDraftsRequest) | [ListSuiteWizardDraftsResponse](#cloud-v1-api-ListSuiteWizardDraftsResponse) | ListSuiteWizardDrafts lists drafts with filtering and pagination. Read-only. |
| PatchSuiteWizard | [PatchSuiteWizardRequest](#cloud-v1-api-PatchSuiteWizardRequest) | [PatchSuiteWizardResponse](#cloud-v1-api-PatchSuiteWizardResponse) | PatchSuiteWizard is idempotent: re-submitting the same form converges. |
| DeleteSuiteWizardDraft | [DeleteSuiteWizardDraftRequest](#cloud-v1-api-DeleteSuiteWizardDraftRequest) | [DeleteSuiteWizardDraftResponse](#cloud-v1-api-DeleteSuiteWizardDraftResponse) | DeleteSuiteWizardDraft is idempotent: deleting an absent draft is a no-op. |
| FinishSuiteWizard | [FinishSuiteWizardRequest](#cloud-v1-api-FinishSuiteWizardRequest) | [FinishSuiteWizardResponse](#cloud-v1-api-FinishSuiteWizardResponse) | FinishSuiteWizard mints a SuiteRun from the draft. Not idempotent. |

 



<a name="cloud_v1_api_system_settings-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/system_settings.proto



<a name="cloud-v1-api-GetSystemSettingsRequest"></a>

### GetSystemSettingsRequest
GetSystemSettingsRequest takes no arguments: it reads the singleton settings.






<a name="cloud-v1-api-GetSystemSettingsResponse"></a>

### GetSystemSettingsResponse
GetSystemSettingsResponse returns the current control-plane settings.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| settings | [PlatformSettings](#cloud-v1-api-PlatformSettings) |  | settings is the singleton platform configuration. |






<a name="cloud-v1-api-UpdateSystemSettingsRequest"></a>

### UpdateSystemSettingsRequest
UpdateSystemSettings replaces the singleton wholesale. Idempotent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| settings | [PlatformSettings](#cloud-v1-api-PlatformSettings) |  | settings is the full replacement platform configuration. |






<a name="cloud-v1-api-UpdateSystemSettingsResponse"></a>

### UpdateSystemSettingsResponse
UpdateSystemSettingsResponse returns the settings after the update.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| settings | [PlatformSettings](#cloud-v1-api-PlatformSettings) |  | settings is the updated singleton platform configuration. |





 

 

 


<a name="cloud-v1-api-SystemSettingsService"></a>

### SystemSettingsService
SystemSettingsService manages the global, singleton control-plane settings.
Admin-only on both read and write.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetSystemSettings | [GetSystemSettingsRequest](#cloud-v1-api-GetSystemSettingsRequest) | [GetSystemSettingsResponse](#cloud-v1-api-GetSystemSettingsResponse) | GetSystemSettings reads the singleton platform settings. Admin-only, read-only. |
| UpdateSystemSettings | [UpdateSystemSettingsRequest](#cloud-v1-api-UpdateSystemSettingsRequest) | [UpdateSystemSettingsResponse](#cloud-v1-api-UpdateSystemSettingsResponse) | UpdateSystemSettings replaces the singleton wholesale. Admin-only. Idempotent. |

 



<a name="cloud_v1_api_tenant_dashboard-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/tenant_dashboard.proto



<a name="cloud-v1-api-GetTenantDashboardRequest"></a>

### GetTenantDashboardRequest
GetTenantDashboardRequest fetches the aggregated dashboard for a tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |






<a name="cloud-v1-api-GetTenantDashboardResponse"></a>

### GetTenantDashboardResponse
GetTenantDashboardResponse returns the computed dashboard payload.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| dashboard | [TenantDashboard](#cloud-v1-api-TenantDashboard) |  | dashboard is the aggregated landing view. |






<a name="cloud-v1-api-StatusCounts"></a>

### StatusCounts
StatusCounts is the run breakdown by lifecycle status.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| total | [uint32](#uint32) |  | total is the count of all runs in scope. |
| pending | [uint32](#uint32) |  | pending is the count of runs not yet started. |
| running | [uint32](#uint32) |  | running is the count of currently executing runs. |
| completed | [uint32](#uint32) |  | completed is the count of successfully finished runs. |
| failed | [uint32](#uint32) |  | failed is the count of runs that ended in failure. |
| cancelled | [uint32](#uint32) |  | cancelled is the count of runs that were cancelled. |






<a name="cloud-v1-api-TenantDashboard"></a>

### TenantDashboard
TenantDashboard is the whole landing payload.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run_counts | [StatusCounts](#cloud-v1-api-StatusCounts) |  | run_counts is the run breakdown by status (for the headline tiles). |
| success_rate | [float](#float) |  | success_rate is completed / finished over a recent window, 0..1. |
| recent_runs | [cloud.v1.models.TestRunRecord](#cloud-v1-models-TestRunRecord) | repeated | recent_runs is the most recent test runs (limited). |
| recent_suite_runs | [cloud.v1.models.SuiteRunRecord](#cloud-v1-models-SuiteRunRecord) | repeated | recent_suite_runs is the most recent suite runs (limited). |
| upcoming | [UpcomingSuite](#cloud-v1-api-UpcomingSuite) | repeated | upcoming is the scheduled suites with their next planned auto-run. |
| top_benchmarks | [RatingEntry](#cloud-v1-api-RatingEntry) | repeated | top_benchmarks is this tenant&#39;s top benchmarks (tenant rating top-N). |






<a name="cloud-v1-api-UpcomingSuite"></a>

### UpcomingSuite
UpcomingSuite is a scheduled suite and its next planned auto-run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| suite_id | [string](#string) |  | suite_id identifies the scheduled suite definition. |
| name | [string](#string) |  | name is the suite&#39;s display name. |
| cron | [string](#string) |  | cron is the schedule expression driving the auto-runs. |
| next_run_at | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | next_run_at is when the next auto-run is planned. |





 

 

 


<a name="cloud-v1-api-TenantDashboardService"></a>

### TenantDashboardService
TenantDashboardService serves the tenant landing-page dashboard.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetTenantDashboard | [GetTenantDashboardRequest](#cloud-v1-api-GetTenantDashboardRequest) | [GetTenantDashboardResponse](#cloud-v1-api-GetTenantDashboardResponse) | GetTenantDashboard returns the aggregated dashboard. Read-only. |

 



<a name="cloud_v1_api_tenant_settings-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/tenant_settings.proto



<a name="cloud-v1-api-GetTenantSettingsRequest"></a>

### GetTenantSettingsRequest
GetTenantSettingsRequest fetches the settings for a tenant.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |






<a name="cloud-v1-api-GetTenantSettingsResponse"></a>

### GetTenantSettingsResponse
GetTenantSettingsResponse returns the tenant&#39;s settings record.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| settings | [cloud.v1.models.TenantSettingsRecord](#cloud-v1-models-TenantSettingsRecord) |  | settings is the tenant&#39;s current settings. |






<a name="cloud-v1-api-SetTenantProviderSettingsRequest"></a>

### SetTenantProviderSettingsRequest
SetTenantProviderSettings sets/replaces the config for ONE provider. The client
sends only the form VALUES (schemapb.Filled = values &#43; schema ref), NOT a full
Baked (which would re-send the schema the server already owns); the server
validates against the provider&#39;s settings schema and bakes the result. Keyed by
`provider`. Idempotent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| provider | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) |  | provider selects which provider&#39;s config to set (must be a defined, non-zero provider). |
| settings | [schemapb.Filled](#schemapb-Filled) |  | settings carries only the form values (Filled = values &#43; schema ref); the server validates against the provider schema and bakes the result. |






<a name="cloud-v1-api-SetTenantProviderSettingsResponse"></a>

### SetTenantProviderSettingsResponse
SetTenantProviderSettingsResponse returns the saved, baked provider config.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| settings | [cloud.v1.deployment.ProviderSettings](#cloud-v1-deployment-ProviderSettings) |  | settings is the saved, baked provider config. |






<a name="cloud-v1-api-UpdateTenantSettingsRequest"></a>

### UpdateTenantSettingsRequest
UpdateTenantSettings replaces the tenant&#39;s settings wholesale. Idempotent.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| settings | [cloud.v1.models.TenantSettingsRecord](#cloud-v1-models-TenantSettingsRecord) |  | settings is the full replacement settings record. |






<a name="cloud-v1-api-UpdateTenantSettingsResponse"></a>

### UpdateTenantSettingsResponse
UpdateTenantSettingsResponse returns the stored settings after the update.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| settings | [cloud.v1.models.TenantSettingsRecord](#cloud-v1-models-TenantSettingsRecord) |  | settings is the tenant&#39;s settings after the update. |





 

 

 


<a name="cloud-v1-api-TenantSettingsService"></a>

### TenantSettingsService
TenantSettingsService is the tenant-scoped API for reading and updating
per-tenant settings and provider configs.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetTenantSettings | [GetTenantSettingsRequest](#cloud-v1-api-GetTenantSettingsRequest) | [GetTenantSettingsResponse](#cloud-v1-api-GetTenantSettingsResponse) | GetTenantSettings fetches the tenant&#39;s settings. Read-only. |
| UpdateTenantSettings | [UpdateTenantSettingsRequest](#cloud-v1-api-UpdateTenantSettingsRequest) | [UpdateTenantSettingsResponse](#cloud-v1-api-UpdateTenantSettingsResponse) | UpdateTenantSettings is idempotent: a wholesale field set converges on retry. |
| SetTenantProviderSettings | [SetTenantProviderSettingsRequest](#cloud-v1-api-SetTenantProviderSettingsRequest) | [SetTenantProviderSettingsResponse](#cloud-v1-api-SetTenantProviderSettingsResponse) | SetTenantProviderSettings is idempotent: setting the same provider config converges. |

 



<a name="cloud_v1_api_test-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/test.proto


 

 

 

 



<a name="cloud_v1_api_test_run-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/test_run.proto



<a name="cloud-v1-api-CancelTestRunRequest"></a>

### CancelTestRunRequest
CancelTestRun requests cancellation (status -&gt; CANCELLING, then CANCELLED when
the workflow stops). Idempotent: cancelling a finished/cancelled run is a no-op.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the test run identifier to cancel. |






<a name="cloud-v1-api-CancelTestRunResponse"></a>

### CancelTestRunResponse
CancelTestRunResponse returns the run after the cancellation request.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run | [cloud.v1.models.TestRunRecord](#cloud-v1-models-TestRunRecord) |  | run is the run record reflecting the cancellation. |






<a name="cloud-v1-api-DeleteTestRunRequest"></a>

### DeleteTestRunRequest
DeleteTestRunRequest deletes a test run by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the test run identifier to delete. |






<a name="cloud-v1-api-DeleteTestRunResponse"></a>

### DeleteTestRunResponse
DeleteTestRunResponse is empty; deletion success is signalled by a non-error
reply.






<a name="cloud-v1-api-ExtractToPresetRequest"></a>

### ExtractToPresetRequest
ExtractToPreset saves a run&#39;s database&#43;workload as a reusable TestPresetRecord,
so a one-off run can be promoted into a named preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the source run to extract the preset from. |
| name | [string](#string) |  | name is the optional name for the new preset; empty -&gt; server derives one. |






<a name="cloud-v1-api-ExtractToPresetResponse"></a>

### ExtractToPresetResponse
ExtractToPresetResponse returns the newly created preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is the saved, reusable test preset. |






<a name="cloud-v1-api-GetTestRunRequest"></a>

### GetTestRunRequest
GetTestRunRequest fetches a single test run by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| id | [string](#string) |  | id is the test run identifier to fetch. |






<a name="cloud-v1-api-GetTestRunResponse"></a>

### GetTestRunResponse
GetTestRunResponse returns the requested test run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run | [cloud.v1.models.TestRunRecord](#cloud-v1-models-TestRunRecord) |  | run is the requested run record. |






<a name="cloud-v1-api-ListTestRunsRequest"></a>

### ListTestRunsRequest
ListTestRunsRequest lists test runs with filtering, faceting, sorting and
pagination.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter holds the shared Entity-level filters (search, ids, author, time windows, soft-delete). |
| statuses | [cloud.v1.common.Status](#cloud-v1-common-Status) | repeated | statuses facets by lifecycle status (all AND; empty list / unset = not applied). |
| db_kinds | [cloud.v1.domain.Database.Kind](#cloud-v1-domain-Database-Kind) | repeated | db_kinds facets by database kind. |
| providers | [cloud.v1.deployment.Provider](#cloud-v1-deployment-Provider) | repeated | providers facets by deployment provider. |
| db_preset_ids | [string](#string) | repeated | db_preset_ids facets by database preset id. |
| workload_preset_ids | [string](#string) | repeated | workload_preset_ids facets by workload preset id. |
| stroppy_versions | [string](#string) | repeated | stroppy_versions facets by stroppy version. |
| suite_run_id | [string](#string) |  | suite_run_id scopes to a single suite run&#39;s children. |
| standalone | [bool](#bool) | optional | standalone facets by suite membership. Unset = both; true = only standalone (no suite); false = only suite children. |
| progress_min | [uint32](#uint32) | optional | progress_min is the lower bound of the progress_pct window, 0..100. |
| progress_max | [uint32](#uint32) | optional | progress_max is the upper bound of the progress_pct window, 0..100. |
| duration_min | [google.protobuf.Duration](#google-protobuf-Duration) |  | duration_min is the lower bound of the duration window. |
| duration_max | [google.protobuf.Duration](#google-protobuf-Duration) |  | duration_max is the upper bound of the duration window. |
| started_after | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_after keeps runs started at or after this time. |
| started_before | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | started_before keeps runs started at or before this time. |
| finished_after | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | finished_after keeps runs finished at or after this time. |
| finished_before | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | finished_before keeps runs finished at or before this time. |
| triggers | [cloud.v1.common.Trigger](#cloud-v1-common-Trigger) | repeated | triggers filters by how the run was triggered (manual / api / suite child); empty = any. |
| sort | [ListTestRunsRequest.Sort](#cloud-v1-api-ListTestRunsRequest-Sort) |  | sort selects the result ordering. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page carries pagination (page size &#43; token). |






<a name="cloud-v1-api-ListTestRunsRequest-Sort"></a>

### ListTestRunsRequest.Sort
Sort orders by a common Entity column OR a denormalized Summary column.
desc applies to whichever is chosen.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| entity | [cloud.v1.common.EntitySortField](#cloud-v1-common-EntitySortField) |  | entity orders by a shared Entity column (name, created_at, ...). |
| kind | [ListTestRunsRequest.Sort.Kind](#cloud-v1-api-ListTestRunsRequest-Sort-Kind) |  | kind orders by a test-run-specific column. |
| desc | [bool](#bool) |  | desc reverses the order (descending) when true. |






<a name="cloud-v1-api-ListTestRunsResponse"></a>

### ListTestRunsResponse
ListTestRunsResponse returns a page of test runs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| runs | [cloud.v1.models.TestRunRecord](#cloud-v1-models-TestRunRecord) | repeated | runs is the matching page of run records. |
| next_page_token | [string](#string) |  | next_page_token fetches the following page; empty when at the end. |






<a name="cloud-v1-api-StartTestRunRequest"></a>

### StartTestRunRequest
StartTestRun launches a run. Provide a fully-baked `run` (CLI / wizard finish)
to persist a new record and start it; or `test_run_id` to re-run an existing
record&#39;s spec as a new run. Launches TestWorkflow.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| run | [cloud.v1.domain.TestRun](#cloud-v1-domain-TestRun) |  | run is a fully-baked TestRun to persist and start (CLI / wizard finish). |
| test_run_id | [string](#string) |  | test_run_id re-runs an existing record&#39;s spec as a new run. |
| in_tenant_rating | [bool](#bool) | optional | in_tenant_rating sets tenant-rating membership; unset -&gt; server defaults (tenant true). |
| in_global_rating | [bool](#bool) | optional | in_global_rating sets global-rating membership; unset -&gt; server defaults (global false). |






<a name="cloud-v1-api-StartTestRunResponse"></a>

### StartTestRunResponse
StartTestRunResponse returns the persisted, launched run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run | [cloud.v1.models.TestRunRecord](#cloud-v1-models-TestRunRecord) |  | run is the persisted, launched run record. |





 


<a name="cloud-v1-api-ListTestRunsRequest-Sort-Kind"></a>

### ListTestRunsRequest.Sort.Kind
Kind is a test-run-specific sortable column (alternative to a common
Entity column).

| Name | Number | Description |
| ---- | ------ | ----------- |
| KIND_UNSPECIFIED | 0 | KIND_UNSPECIFIED leaves the test-run-specific ordering unset. |
| KIND_STATUS | 1 | KIND_STATUS orders by lifecycle status. |
| KIND_DB_KIND | 2 | KIND_DB_KIND orders by database kind. |
| KIND_WORKLOAD | 3 | KIND_WORKLOAD orders by workload_name / stroppy_version. |
| KIND_PROVIDER | 4 | KIND_PROVIDER orders by deployment provider. |
| KIND_PROGRESS | 5 | KIND_PROGRESS orders by progress. |
| KIND_DURATION | 6 | KIND_DURATION orders by run duration. |
| KIND_STARTED_AT | 7 | KIND_STARTED_AT orders by start time. |
| KIND_FINISHED_AT | 8 | KIND_FINISHED_AT orders by finish time. |
| KIND_NODE_COUNT | 9 | KIND_NODE_COUNT orders by node count. |


 

 


<a name="cloud-v1-api-TestRunService"></a>

### TestRunService
TestRunService is the tenant-scoped lifecycle API for test runs.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| StartTestRun | [StartTestRunRequest](#cloud-v1-api-StartTestRunRequest) | [StartTestRunResponse](#cloud-v1-api-StartTestRunResponse) | StartTestRun is not idempotent: each call launches a new run. |
| GetTestRun | [GetTestRunRequest](#cloud-v1-api-GetTestRunRequest) | [GetTestRunResponse](#cloud-v1-api-GetTestRunResponse) | GetTestRun fetches a single run by id. Read-only. |
| ListTestRuns | [ListTestRunsRequest](#cloud-v1-api-ListTestRunsRequest) | [ListTestRunsResponse](#cloud-v1-api-ListTestRunsResponse) | ListTestRuns lists runs with filtering and pagination. Read-only. |
| CancelTestRun | [CancelTestRunRequest](#cloud-v1-api-CancelTestRunRequest) | [CancelTestRunResponse](#cloud-v1-api-CancelTestRunResponse) | CancelTestRun is idempotent: cancelling a finished/cancelled run is a no-op. |
| DeleteTestRun | [DeleteTestRunRequest](#cloud-v1-api-DeleteTestRunRequest) | [DeleteTestRunResponse](#cloud-v1-api-DeleteTestRunResponse) | DeleteTestRun is idempotent: deleting an absent run is a no-op. |
| ExtractToPreset | [ExtractToPresetRequest](#cloud-v1-api-ExtractToPresetRequest) | [ExtractToPresetResponse](#cloud-v1-api-ExtractToPresetResponse) | ExtractToPreset mints a new preset. Not idempotent. |

 



<a name="cloud_v1_api_test_run_overview-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/test_run_overview.proto



<a name="cloud-v1-api-GetRunMetricsRequest"></a>

### GetRunMetricsRequest
GetRunMetricsRequest fetches the Metrics tab payload for a run. Multi-run
comparison lives in compare.proto (CompareService.CompareRuns).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| run_id | [string](#string) |  | run_id identifies the run whose metrics are requested. |






<a name="cloud-v1-api-GetRunMetricsResponse"></a>

### GetRunMetricsResponse
GetRunMetricsResponse returns the run&#39;s metrics.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metrics | [cloud.v1.monitor.RunMetrics](#cloud-v1-monitor-RunMetrics) |  | metrics is the run&#39;s aggregated metrics. |






<a name="cloud-v1-api-GetTestRunOverviewRequest"></a>

### GetTestRunOverviewRequest
GetTestRunOverviewRequest fetches the Overview tab payload for a run.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| run_id | [string](#string) |  | run_id identifies the run whose overview is requested. |






<a name="cloud-v1-api-GetTestRunOverviewResponse"></a>

### GetTestRunOverviewResponse
GetTestRunOverviewResponse returns the run&#39;s Overview snapshot.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| overview | [cloud.v1.monitor.Overview](#cloud-v1-monitor-Overview) |  | overview is the full overview (status/pipeline/workers/timeline). |






<a name="cloud-v1-api-LogFilter"></a>

### LogFilter
LogFilter is the flexible log filter shared by query and stream. All fields are
optional and AND-combined. `query` is the raw-LogsQL escape hatch for advanced
filtering beyond the structured fields.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| node_execution_ids | [string](#string) | repeated | node_execution_ids restricts to lines emitted by these node executions. |
| component_ids | [string](#string) | repeated | component_ids restricts to lines emitted by these components. |
| machine_ids | [string](#string) | repeated | machine_ids restricts to lines emitted by these machines. |
| sources | [cloud.v1.monitor.Source](#cloud-v1-monitor-Source) | repeated | sources restricts to these log sources. |
| streams | [cloud.v1.monitor.Stream](#cloud-v1-monitor-Stream) | repeated | streams restricts to these log streams (e.g. stdout/stderr). |
| unit | [string](#string) |  | unit restricts to a specific systemd/log unit. |
| start | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | start keeps lines at or after this time. |
| end | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | end keeps lines at or before this time. |
| search | [string](#string) |  | search is a simple substring match over the line text. |
| query | [string](#string) |  | query is a raw LogsQL fragment, AND-ed with the structured filters (advanced). |






<a name="cloud-v1-api-QueryLogsRequest"></a>

### QueryLogsRequest
QueryLogsRequest fetches a bounded, cursor-paged window of historical log lines.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| run_id | [string](#string) |  | run_id identifies the run whose logs are queried. |
| filter | [LogFilter](#cloud-v1-api-LogFilter) |  | filter narrows the lines returned. |
| from | [cloud.v1.monitor.LogCursor](#cloud-v1-monitor-LogCursor) |  | from is the anchor to page from; empty = newest (when OLDER) / oldest (when NEWER). |
| direction | [LogScrollDirection](#cloud-v1-api-LogScrollDirection) |  | direction selects which way to page relative to the anchor. |
| limit | [uint32](#uint32) |  | limit caps the lines returned; 0 -&gt; server default. Bounded so the UI never drowns. |






<a name="cloud-v1-api-QueryLogsResponse"></a>

### QueryLogsResponse
QueryLogsResponse returns a page of log lines plus cursors to page either way.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| lines | [cloud.v1.monitor.LogLine](#cloud-v1-monitor-LogLine) | repeated | lines is the matching page of log lines. |
| older | [cloud.v1.monitor.LogCursor](#cloud-v1-monitor-LogCursor) |  | older is the cursor to fetch the page OLDER than these results (empty = at the start). |
| newer | [cloud.v1.monitor.LogCursor](#cloud-v1-monitor-LogCursor) |  | newer is the cursor to fetch the page NEWER than these results (empty = at the end / tip). |






<a name="cloud-v1-api-ResolveLogRefRequest"></a>

### ResolveLogRefRequest
ResolveLogRef turns a shareable LogRef (deep-link, e.g. built from a pipeline
node) into a concrete filter &#43; anchor cursor, so opening a link lands every
user on the exact same place.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| ref | [cloud.v1.monitor.LogRef](#cloud-v1-monitor-LogRef) |  | ref is the shareable log reference (deep-link) to resolve. |






<a name="cloud-v1-api-ResolveLogRefResponse"></a>

### ResolveLogRefResponse
ResolveLogRefResponse returns the run id, filter and anchor cursor the LogRef
resolves to.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| run_id | [string](#string) |  | run_id is the run the ref points at. |
| filter | [LogFilter](#cloud-v1-api-LogFilter) |  | filter is the concrete filter the ref resolves to. |
| cursor | [cloud.v1.monitor.LogCursor](#cloud-v1-monitor-LogCursor) |  | cursor is the anchor cursor to land on. |






<a name="cloud-v1-api-StreamLogsRequest"></a>

### StreamLogsRequest
StreamLogsRequest opens a live log tail. StreamLogs returns a live stream of
LogLine; the server MAY coalesce / rate-limit under very high throughput. The
client uses QueryLogs for exact scrollback.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| run_id | [string](#string) |  | run_id identifies the run to tail. |
| filter | [LogFilter](#cloud-v1-api-LogFilter) |  | filter is the live filter (the user can re-subscribe with a new filter to filter the tail). |
| from | [cloud.v1.monitor.LogCursor](#cloud-v1-monitor-LogCursor) |  | from is an optional start position; empty = from now (tail). Set to backfill from a point. |






<a name="cloud-v1-api-StreamTestRunOverviewRequest"></a>

### StreamTestRunOverviewRequest
StreamTestRunOverviewRequest opens a live overview stream for a run. Each stream
tick is a fresh full Overview (status/pipeline/workers/timeline) so the client
just replaces its state; no diff merging.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| run_id | [string](#string) |  | run_id identifies the run to follow. |





 


<a name="cloud-v1-api-LogScrollDirection"></a>

### LogScrollDirection
LogScrollDirection selects which way QueryLogs pages relative to the anchor.

| Name | Number | Description |
| ---- | ------ | ----------- |
| LOG_SCROLL_DIRECTION_UNSPECIFIED | 0 | LOG_SCROLL_DIRECTION_UNSPECIFIED uses the server default (newer). |
| LOG_SCROLL_DIRECTION_OLDER | 1 | LOG_SCROLL_DIRECTION_OLDER pages back in time. |
| LOG_SCROLL_DIRECTION_NEWER | 2 | LOG_SCROLL_DIRECTION_NEWER pages forward in time. |


 

 


<a name="cloud-v1-api-TestRunOverviewService"></a>

### TestRunOverviewService
TestRunOverviewService serves the read-only overview, logs and metrics for a
single run.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| GetTestRunOverview | [GetTestRunOverviewRequest](#cloud-v1-api-GetTestRunOverviewRequest) | [GetTestRunOverviewResponse](#cloud-v1-api-GetTestRunOverviewResponse) | GetTestRunOverview fetches the Overview snapshot. Read-only. |
| StreamTestRunOverview | [StreamTestRunOverviewRequest](#cloud-v1-api-StreamTestRunOverviewRequest) | [.cloud.v1.monitor.Overview](#cloud-v1-monitor-Overview) stream | StreamTestRunOverview streams full Overview snapshots as the run progresses. |
| QueryLogs | [QueryLogsRequest](#cloud-v1-api-QueryLogsRequest) | [QueryLogsResponse](#cloud-v1-api-QueryLogsResponse) | QueryLogs fetches a cursor-paged window of historical log lines. Read-only. |
| StreamLogs | [StreamLogsRequest](#cloud-v1-api-StreamLogsRequest) | [.cloud.v1.monitor.LogLine](#cloud-v1-monitor-LogLine) stream | StreamLogs streams a live log tail (server may coalesce/rate-limit). |
| ResolveLogRef | [ResolveLogRefRequest](#cloud-v1-api-ResolveLogRefRequest) | [ResolveLogRefResponse](#cloud-v1-api-ResolveLogRefResponse) | ResolveLogRef resolves a shareable LogRef into a filter &#43; anchor. Read-only. |
| GetRunMetrics | [GetRunMetricsRequest](#cloud-v1-api-GetRunMetricsRequest) | [GetRunMetricsResponse](#cloud-v1-api-GetRunMetricsResponse) | GetRunMetrics fetches the run&#39;s metrics. Read-only. |

 



<a name="cloud_v1_api_test_wizard-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## cloud/v1/api/test_wizard.proto



<a name="cloud-v1-api-DeleteTestWizardDraftRequest"></a>

### DeleteTestWizardDraftRequest
DeleteTestWizardDraftRequest deletes a wizard draft by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft identifier to delete. |






<a name="cloud-v1-api-DeleteTestWizardDraftResponse"></a>

### DeleteTestWizardDraftResponse
DeleteTestWizardDraftResponse is empty; deletion success is signalled by a
non-error reply.






<a name="cloud-v1-api-FinishTestWizardRequest"></a>

### FinishTestWizardRequest
FinishTestWizard bakes the form (Filled -&gt; Baked, layered overrides applied)
into a ready domain.TestRun. Rejected unless draft.ready. Optionally, in the
same call: start it (internally calls TestRunAPI.StartTestRun -&gt; persists a
TestRunRecord &#43; launches TestWorkflow) and/or save it as a TestPresetRecord.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft to bake. |
| start | [bool](#bool) |  | start, when true, launches the baked run immediately. |
| save_as_preset | [bool](#bool) |  | save_as_preset, when true, also saves the baked db&#43;workload as a reusable test preset. |
| preset_name | [string](#string) |  | preset_name names the saved preset (used only when save_as_preset; empty -&gt; derived). |
| in_tenant_rating | [bool](#bool) | optional | in_tenant_rating sets tenant-rating membership for the started run; unset -&gt; defaults (tenant true). |
| in_global_rating | [bool](#bool) | optional | in_global_rating sets global-rating membership for the started run; unset -&gt; defaults (global false). |






<a name="cloud-v1-api-FinishTestWizardResponse"></a>

### FinishTestWizardResponse
FinishTestWizardResponse returns the baked run and, optionally, the launched run
and saved preset.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| test_run | [cloud.v1.domain.TestRun](#cloud-v1-domain-TestRun) |  | test_run is the baked run spec (always present). |
| run | [cloud.v1.models.TestRunRecord](#cloud-v1-models-TestRunRecord) |  | run is set when start = true: the persisted, launched run. |
| preset | [cloud.v1.models.TestPresetRecord](#cloud-v1-models-TestPresetRecord) |  | preset is set when save_as_preset = true: the saved preset. |






<a name="cloud-v1-api-GetTestWizardDraftRequest"></a>

### GetTestWizardDraftRequest
GetTestWizardDraftRequest fetches a single wizard draft by id.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft identifier to fetch. |






<a name="cloud-v1-api-GetTestWizardDraftResponse"></a>

### GetTestWizardDraftResponse
GetTestWizardDraftResponse returns the requested wizard draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| draft | [cloud.v1.models.TestWizardDraftRecord](#cloud-v1-models-TestWizardDraftRecord) |  | draft is the requested wizard draft. |






<a name="cloud-v1-api-ListTestWizardDraftsRequest"></a>

### ListTestWizardDraftsRequest
ListTestWizardDrafts lets the UI offer &#34;continue where you left off&#34;: filter by
author (the caller) and sort by updated_at desc to pick the most recent draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| filter | [cloud.v1.common.EntityFilter](#cloud-v1-common-EntityFilter) |  | filter holds the shared Entity-level filters (author, time windows, ...). |
| sort | [cloud.v1.common.EntitySort](#cloud-v1-common-EntitySort) |  | sort selects the result ordering. |
| page | [cloud.v1.common.Page](#cloud-v1-common-Page) |  | page carries pagination (page size &#43; token). |






<a name="cloud-v1-api-ListTestWizardDraftsResponse"></a>

### ListTestWizardDraftsResponse
ListTestWizardDraftsResponse returns a page of wizard drafts.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| drafts | [cloud.v1.models.TestWizardDraftRecord](#cloud-v1-models-TestWizardDraftRecord) | repeated | drafts is the matching page of wizard drafts. |
| next_page_token | [string](#string) |  | next_page_token fetches the following page; empty when at the end. |






<a name="cloud-v1-api-PatchTestWizardRequest"></a>

### PatchTestWizardRequest
PatchTestWizard submits the edited form. The server validates the whole schema
(honoring `when` gates), regenerates the topology and recomputes readiness,
returning the full new draft (form may carry a re-emitted schema when a coarse
choice changed the active branches).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| draft_id | [string](#string) |  | draft_id is the wizard draft being edited. |
| form | [schemapb.Filled](#schemapb-Filled) |  | form carries the edited form values (Filled = values &#43; schema ref). |






<a name="cloud-v1-api-PatchTestWizardResponse"></a>

### PatchTestWizardResponse
PatchTestWizardResponse returns the recomputed wizard draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| draft | [cloud.v1.models.TestWizardDraftRecord](#cloud-v1-models-TestWizardDraftRecord) |  | draft is the full new draft after validation/topology/readiness recompute. |






<a name="cloud-v1-api-StartTestWizardRequest"></a>

### StartTestWizardRequest
StartTestWizardRequest opens a new test wizard draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tenant_id | [string](#string) |  | tenant_id scopes the request to the owning tenant. |
| name | [string](#string) |  | name is the optional human label for the draft. |
| test_preset_id | [string](#string) |  | test_preset_id optionally seeds the whole draft from an existing test preset. |






<a name="cloud-v1-api-StartTestWizardResponse"></a>

### StartTestWizardResponse
StartTestWizardResponse returns the freshly created draft.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| draft | [cloud.v1.models.TestWizardDraftRecord](#cloud-v1-models-TestWizardDraftRecord) |  | draft is the new wizard draft (carrying the initial form schema). |





 

 

 


<a name="cloud-v1-api-TestWizardService"></a>

### TestWizardService
TestWizardService drives the test wizard: start, fetch/list, patch-loop and
finish into a baked TestRun (optionally started and/or saved as a preset).

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| StartTestWizard | [StartTestWizardRequest](#cloud-v1-api-StartTestWizardRequest) | [StartTestWizardResponse](#cloud-v1-api-StartTestWizardResponse) | StartTestWizard opens a new draft. Not idempotent. |
| GetTestWizardDraft | [GetTestWizardDraftRequest](#cloud-v1-api-GetTestWizardDraftRequest) | [GetTestWizardDraftResponse](#cloud-v1-api-GetTestWizardDraftResponse) | GetTestWizardDraft fetches a single draft by id. Read-only. |
| ListTestWizardDrafts | [ListTestWizardDraftsRequest](#cloud-v1-api-ListTestWizardDraftsRequest) | [ListTestWizardDraftsResponse](#cloud-v1-api-ListTestWizardDraftsResponse) | ListTestWizardDrafts lists drafts with filtering and pagination. Read-only. |
| PatchTestWizard | [PatchTestWizardRequest](#cloud-v1-api-PatchTestWizardRequest) | [PatchTestWizardResponse](#cloud-v1-api-PatchTestWizardResponse) | PatchTestWizard is idempotent: re-submitting the same form converges. |
| DeleteTestWizardDraft | [DeleteTestWizardDraftRequest](#cloud-v1-api-DeleteTestWizardDraftRequest) | [DeleteTestWizardDraftResponse](#cloud-v1-api-DeleteTestWizardDraftResponse) | DeleteTestWizardDraft is idempotent: deleting an absent draft is a no-op. |
| FinishTestWizard | [FinishTestWizardRequest](#cloud-v1-api-FinishTestWizardRequest) | [FinishTestWizardResponse](#cloud-v1-api-FinishTestWizardResponse) | FinishTestWizard mints a TestRun from the draft. Not idempotent. |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

