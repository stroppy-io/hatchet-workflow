// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"github.com/stephenafamo/bob/clause"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
)

var (
	SelectWhere     = Where[*dialect.SelectQuery]()
	UpdateWhere     = Where[*dialect.UpdateQuery]()
	DeleteWhere     = Where[*dialect.DeleteQuery]()
	OnConflictWhere = Where[*clause.ConflictClause]() // Used in ON CONFLICT DO UPDATE
)

func Where[Q psql.Filterable]() struct {
	TestRunRecords          testRunRecordWhere[Q]
	TestWizardDrafts        testWizardDraftWhere[Q]
	TestPresetRecords       testPresetRecordWhere[Q]
	PlatformSettings        platformSettingWhere[Q]
	SuiteRecords            suiteRecordWhere[Q]
	SuiteRunRecords         suiteRunRecordWhere[Q]
	SuiteWizardDrafts       suiteWizardDraftWhere[Q]
	ShareRecords            shareRecordWhere[Q]
	FavoriteRecords         favoriteRecordWhere[Q]
	PackageRecords          packageRecordWhere[Q]
	DatabasePresetRecords   databasePresetRecordWhere[Q]
	WorkloadPresetRecords   workloadPresetRecordWhere[Q]
	TenantSettingsRecords   tenantSettingsRecordWhere[Q]
	IamAccounts             iamAccountWhere[Q]
	IamCredentials          iamCredentialWhere[Q]
	IamAPITokens            iamAPITokenWhere[Q]
	IamTenants              iamTenantWhere[Q]
	IamRoles                iamRoleWhere[Q]
	IamMemberships          iamMembershipWhere[Q]
	IamIdentityProviders    iamIdentityProviderWhere[Q]
	IamExternalIdentities   iamExternalIdentityWhere[Q]
	IamOneTimeTokens        iamOneTimeTokenWhere[Q]
	IdentitySecrets         identitySecretWhere[Q]
	IdentityRefreshSessions identityRefreshSessionWhere[Q]
	IdentitySsoStates       identitySsoStateWhere[Q]
} {
	return struct {
		TestRunRecords          testRunRecordWhere[Q]
		TestWizardDrafts        testWizardDraftWhere[Q]
		TestPresetRecords       testPresetRecordWhere[Q]
		PlatformSettings        platformSettingWhere[Q]
		SuiteRecords            suiteRecordWhere[Q]
		SuiteRunRecords         suiteRunRecordWhere[Q]
		SuiteWizardDrafts       suiteWizardDraftWhere[Q]
		ShareRecords            shareRecordWhere[Q]
		FavoriteRecords         favoriteRecordWhere[Q]
		PackageRecords          packageRecordWhere[Q]
		DatabasePresetRecords   databasePresetRecordWhere[Q]
		WorkloadPresetRecords   workloadPresetRecordWhere[Q]
		TenantSettingsRecords   tenantSettingsRecordWhere[Q]
		IamAccounts             iamAccountWhere[Q]
		IamCredentials          iamCredentialWhere[Q]
		IamAPITokens            iamAPITokenWhere[Q]
		IamTenants              iamTenantWhere[Q]
		IamRoles                iamRoleWhere[Q]
		IamMemberships          iamMembershipWhere[Q]
		IamIdentityProviders    iamIdentityProviderWhere[Q]
		IamExternalIdentities   iamExternalIdentityWhere[Q]
		IamOneTimeTokens        iamOneTimeTokenWhere[Q]
		IdentitySecrets         identitySecretWhere[Q]
		IdentityRefreshSessions identityRefreshSessionWhere[Q]
		IdentitySsoStates       identitySsoStateWhere[Q]
	}{
		TestRunRecords:          buildTestRunRecordWhere[Q](TestRunRecords.Columns),
		TestWizardDrafts:        buildTestWizardDraftWhere[Q](TestWizardDrafts.Columns),
		TestPresetRecords:       buildTestPresetRecordWhere[Q](TestPresetRecords.Columns),
		PlatformSettings:        buildPlatformSettingWhere[Q](PlatformSettings.Columns),
		SuiteRecords:            buildSuiteRecordWhere[Q](SuiteRecords.Columns),
		SuiteRunRecords:         buildSuiteRunRecordWhere[Q](SuiteRunRecords.Columns),
		SuiteWizardDrafts:       buildSuiteWizardDraftWhere[Q](SuiteWizardDrafts.Columns),
		ShareRecords:            buildShareRecordWhere[Q](ShareRecords.Columns),
		FavoriteRecords:         buildFavoriteRecordWhere[Q](FavoriteRecords.Columns),
		PackageRecords:          buildPackageRecordWhere[Q](PackageRecords.Columns),
		DatabasePresetRecords:   buildDatabasePresetRecordWhere[Q](DatabasePresetRecords.Columns),
		WorkloadPresetRecords:   buildWorkloadPresetRecordWhere[Q](WorkloadPresetRecords.Columns),
		TenantSettingsRecords:   buildTenantSettingsRecordWhere[Q](TenantSettingsRecords.Columns),
		IamAccounts:             buildIamAccountWhere[Q](IamAccounts.Columns),
		IamCredentials:          buildIamCredentialWhere[Q](IamCredentials.Columns),
		IamAPITokens:            buildIamAPITokenWhere[Q](IamAPITokens.Columns),
		IamTenants:              buildIamTenantWhere[Q](IamTenants.Columns),
		IamRoles:                buildIamRoleWhere[Q](IamRoles.Columns),
		IamMemberships:          buildIamMembershipWhere[Q](IamMemberships.Columns),
		IamIdentityProviders:    buildIamIdentityProviderWhere[Q](IamIdentityProviders.Columns),
		IamExternalIdentities:   buildIamExternalIdentityWhere[Q](IamExternalIdentities.Columns),
		IamOneTimeTokens:        buildIamOneTimeTokenWhere[Q](IamOneTimeTokens.Columns),
		IdentitySecrets:         buildIdentitySecretWhere[Q](IdentitySecrets.Columns),
		IdentityRefreshSessions: buildIdentityRefreshSessionWhere[Q](IdentityRefreshSessions.Columns),
		IdentitySsoStates:       buildIdentitySsoStateWhere[Q](IdentitySsoStates.Columns),
	}
}
