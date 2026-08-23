Feature: Account profile (GET /api/me)

  User provisioning happens just-in-time in the auth.Authenticator middleware,
  on *every* authenticated request. GET /api/me is therefore a read of
  already-provisioned state and doubles as the client's source of truth for
  the current tier.

  Scenario: An anonymous request is rejected
    Given I am anonymous
    When I make a GET call to /api/me
    Then I should receive a 401 response

  Scenario: An authenticated caller sees their own freshly provisioned profile
    Given I am authenticated as "alice"
    When I make a GET call to /api/me
    Then I should receive a 200 JSON response
    And the response field "id" should match "^usr_[a-z0-9]{26}$"
    And the response field "email" should be "alice@example.test"
    And the response field "tier" should be "free"

  Scenario: A brand-new identity is provisioned by its very first call
    # No prior request has ever been made for this identity in this scenario's
    # (freshly truncated) database, so the auth middleware must create the
    # user row inline, before the handler runs, not GET /me itself.
    Given I am authenticated as "brand-new-birder"
    When I make a GET call to /api/me
    Then I should receive a 200 JSON response
    And the response field "email" should be "brand-new-birder@example.test"
    And the response field "tier" should be "free"

  Scenario: A premium tier is reflected on the profile
    Given I am authenticated as "carol"
    And the user "carol" has tier "premium"
    When I make a GET call to /api/me
    Then I should receive a 200 JSON response
    And the response field "tier" should be "premium"
