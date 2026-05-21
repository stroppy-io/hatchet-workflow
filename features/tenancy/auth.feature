# language: en
@tenancy @migration
Feature: Authentication and tokens (A)

  Login issues an access+refresh pair (previously a single JWT). Access is a
  short-lived stateless JWT; refresh/sessions live in Valkey with a TTL (G3), so
  that logout/revocation work. Login by email or nickname. Canon: api/ui/auth.proto,
  models/account.proto.

  Scenario: Login issues a TokenPair
    Given an account with email and password
    When Login(email|nickname, password) with the correct password
    Then a TokenPair{access, refresh, expires_in} is returned
    And the refresh session is written to Valkey with a TTL

  Scenario: Wrong password is rejected
    When Login with the wrong password
    Then denied (bcrypt CheckPassword failed)

  Scenario: Login by nickname or email
    Given an account with email "a@b.c" and nickname "alice"
    When Login with "alice" or "a@b.c"
    Then the server resolves the right column and logs in

  Scenario: Refresh issues a new pair
    Given a valid refresh (cookie or explicit token)
    When RefreshTokens
    Then a new TokenPair
    And the old refresh session in Valkey is rotated

  Scenario: Logout revokes the refresh
    When Logout
    Then the refresh session is removed from Valkey
    And a subsequent refresh with that token is rejected

  Scenario: Me returns the caller's account
    Given a valid access token
    When Me
    Then the caller's Account is returned

  # --- error / edge cases ---

  Scenario: An invalid or tampered access token is rejected
    Given a malformed or forged JWT
    When a request presents it
    Then authentication fails (invalid credentials)

  Scenario: An expired access token is rejected
    Given an access token past its expiry
    When it is presented
    Then authentication fails

  Scenario: A reused (rotated) refresh token is rejected
    Given a refresh token already rotated by a prior RefreshTokens
    When it is reused
    Then it is rejected (rotation invalidated it)

  Scenario: An action above the caller's role is denied
    Given a tenant member whose role is below what the action requires
    When they attempt it
    Then it is denied (insufficient TenantMember.Role)
