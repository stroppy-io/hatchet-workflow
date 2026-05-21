# language: en
@tenancy @migration
Feature: Tenant API tokens (A) — programmatic CI/SDK auth

  API tokens are the third auth principal (alongside Account login JWT and the agent
  machine JWT): a long-lived, tenant-scoped Bearer credential. Recast of the old
  tenant_api_tokens. Only the sha256 hash is stored; the plaintext is shown once.
  Role is capped <= ADMIN (never OWNER). Minted/revoked by OWNER only. Canon:
  models/apitoken.proto, api/ui/apitoken.proto.

  Scenario: OWNER mints an API token, secret returned once
    Given a tenant OWNER
    When they call ApiTokenService.CreateApiToken with name and role ADMIN
    Then a CreateApiTokenResponse is returned with the plaintext secret
    And only the sha256 hash is persisted (token_hash, write-only)
    And the secret is never returned again

  Scenario: Non-OWNER cannot mint API tokens
    Given a tenant member with role VIEWER or ADMIN
    When they call CreateApiToken
    Then denied (only OWNER mints tenant API tokens)

  Scenario Outline: token role is capped to <= ADMIN
    When CreateApiToken with role <role>
    Then the result = <result>

    Examples:
      | role   | result   |
      | VIEWER | accepted |
      | ADMIN  | accepted |
      | OWNER  | rejected |

  Scenario: Authenticating with an API token
    Given a valid API token as a Bearer credential
    When a request arrives and is not a JWT
    Then the server looks up sha256(token) in api_tokens (not expired)
    And authenticates as principal {tenant_id, role}

  Scenario: Expired or revoked token is rejected
    Given an API token past its expires_at, or revoked
    When it is presented
    Then authentication fails

  Scenario: List and revoke
    Given a tenant OWNER
    When they call ListApiTokens
    Then token metadata is returned (no secret)
    When they call RevokeApiToken by id
    Then the token no longer authenticates

  Scenario: CLI uses an API token for headless auth
    Given a CI pipeline with an API token (G7)
    When the CLI calls the API with the Bearer token
    Then it acts within the token's tenant and capped role
