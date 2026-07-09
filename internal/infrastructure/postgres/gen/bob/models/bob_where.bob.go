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
	RunRecords              runRecordWhere[Q]
	PlatformSettings        platformSettingWhere[Q]
	RegistrationRequests    registrationRequestWhere[Q]
	RecipeRecords           recipeRecordWhere[Q]
	CatalogEntries          catalogEntryWhere[Q]
	ShareRecords            shareRecordWhere[Q]
	FavoriteRecords         favoriteRecordWhere[Q]
	PackageRecords          packageRecordWhere[Q]
	TenantSettingsRecords   tenantSettingsRecordWhere[Q]
	QuotaSnapshots          quotaSnapshotWhere[Q]
	QuotaReservations       quotaReservationWhere[Q]
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
		RunRecords              runRecordWhere[Q]
		PlatformSettings        platformSettingWhere[Q]
		RegistrationRequests    registrationRequestWhere[Q]
		RecipeRecords           recipeRecordWhere[Q]
		CatalogEntries          catalogEntryWhere[Q]
		ShareRecords            shareRecordWhere[Q]
		FavoriteRecords         favoriteRecordWhere[Q]
		PackageRecords          packageRecordWhere[Q]
		TenantSettingsRecords   tenantSettingsRecordWhere[Q]
		QuotaSnapshots          quotaSnapshotWhere[Q]
		QuotaReservations       quotaReservationWhere[Q]
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
		RunRecords:              buildRunRecordWhere[Q](RunRecords.Columns),
		PlatformSettings:        buildPlatformSettingWhere[Q](PlatformSettings.Columns),
		RegistrationRequests:    buildRegistrationRequestWhere[Q](RegistrationRequests.Columns),
		RecipeRecords:           buildRecipeRecordWhere[Q](RecipeRecords.Columns),
		CatalogEntries:          buildCatalogEntryWhere[Q](CatalogEntries.Columns),
		ShareRecords:            buildShareRecordWhere[Q](ShareRecords.Columns),
		FavoriteRecords:         buildFavoriteRecordWhere[Q](FavoriteRecords.Columns),
		PackageRecords:          buildPackageRecordWhere[Q](PackageRecords.Columns),
		TenantSettingsRecords:   buildTenantSettingsRecordWhere[Q](TenantSettingsRecords.Columns),
		QuotaSnapshots:          buildQuotaSnapshotWhere[Q](QuotaSnapshots.Columns),
		QuotaReservations:       buildQuotaReservationWhere[Q](QuotaReservations.Columns),
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
