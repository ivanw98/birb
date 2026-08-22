Feature: Joining a group by code (POST /api/groups/join)

  Background:
    Given a group "Walkers" owned by "bob" exists with join code "BQTX7RKM"
    And I am authenticated as "alice" with display name "Alice"

  Scenario: Joining by the exact code returns the group with both members
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 200 JSON response
    And the response field "name" should be "Walkers"
    And the response field "isOwner" should be "false"
    And the response field "members" should have 2 items

  Scenario: A lowercase code is normalised
    When I make a POST call to /api/groups/join with body
      """
      { "code": "bqtx7rkm" }
      """
    Then I should receive a 200 JSON response
    And the response field "name" should be "Walkers"

  Scenario: A hyphenated code is normalised
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX-7RKM" }
      """
    Then I should receive a 200 JSON response
    And the response field "name" should be "Walkers"

  Scenario: A space-padded code is normalised
    When I make a POST call to /api/groups/join with body
      """
      { "code": "  bqtx 7rkm  " }
      """
    Then I should receive a 200 JSON response
    And the response field "name" should be "Walkers"

  Scenario: Re-joining a group I am already in is idempotent
    Given "alice" is a member of group "Walkers"
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 200 JSON response
    And the response field "members" should have 2 items

  Scenario: The owner re-joining their own group is idempotent
    Given I am authenticated as "bob"
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 200 JSON response
    And the response field "isOwner" should be "true"
    And the response field "members" should have 1 items

  Scenario: An unknown code is refused
    When I make a POST call to /api/groups/join with body
      """
      { "code": "ZZZZZZZZ" }
      """
    Then I should receive a 404 JSON response
    And the response field "code" should be "unknown_join_code"

  Scenario: A too-short code is refused the same way as an unknown one
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RK" }
      """
    Then I should receive a 404 JSON response
    And the response field "code" should be "unknown_join_code"

  Scenario: A code containing an excluded character is refused, not silently shifted
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKO" }
      """
    Then I should receive a 404 JSON response
    And the response field "code" should be "unknown_join_code"

  Scenario: An over-length code is rejected as invalid input
    Given a string of 40 "A" characters is saved as "huge_code"
    When I make a POST call to /api/groups/join with body
      """
      { "code": "{{ huge_code }}" }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: Joining a full group is refused
    Given group "Walkers" has 24 other members
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 409 JSON response
    And the response field "code" should be "group_full"

  Scenario: Re-joining a full group I am already in still succeeds
    Given "alice" is a member of group "Walkers"
    And group "Walkers" has 23 other members
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 200 JSON response
    And the response field "members" should have 25 items

  Scenario: Joining at the membership cap is refused
    Given "alice" is a member of 10 groups
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 409 JSON response
    And the response field "code" should be "group_limit_reached"

  Scenario: Re-joining at the membership cap still succeeds
    Given "alice" is a member of group "Walkers"
    And "alice" is a member of 9 groups
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 200 JSON response
    And the response field "name" should be "Walkers"

  Scenario: Repeated failures are rate limited
    Given I make 2 failed join attempts
    When I make a POST call to /api/groups/join with body
      """
      { "code": "ZZZZZZZZ" }
      """
    Then I should receive a 429 JSON response
    And the response field "code" should be "join_rate_limited"

  Scenario: Successful joins do not count towards the failure limit
    Given a group "Second" owned by "bob" exists with join code "MNPQRSTV"
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    And I make a POST call to /api/groups/join with body
      """
      { "code": "MNPQRSTV" }
      """
    And I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 200 JSON response

  Scenario: An anonymous caller cannot join
    Given I am anonymous
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 401 JSON response
