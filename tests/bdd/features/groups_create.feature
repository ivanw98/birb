Feature: Creating a group (POST /api/groups)

  The caller becomes owner and first member; the server mints the join code.
  Not idempotent, so a caller at the owned-groups cap is refused.

  Background:
    Given I am authenticated as "alice" with display name "Alice"

  Scenario: Creating a group returns it with the caller as sole owner-member
    When I make a POST call to /api/groups with body
      """
      { "name": "Sunday walkers" }
      """
    Then I should receive a 201 JSON response
    And the response field "name" should be "Sunday walkers"
    And the response field "isOwner" should be "true"
    And the response field "members" should have 1 items
    And the response field "members.0.isOwner" should be "true"
    And the response field "members.0.name" should be "Alice"

  Scenario: The new group carries a well-formed id and join code
    When I make a POST call to /api/groups with body
      """
      { "name": "Sunday walkers" }
      """
    Then I should receive a 201 JSON response
    And the response field "id" should match "^grp_[a-z0-9]{26}$"
    And the response field "joinCode" should match "^[ABCDEFGHJKMNPQRSTVWXYZ23456789]{8}$"

  Scenario: A padded name is trimmed before it is stored
    When I make a POST call to /api/groups with body
      """
      { "name": "   Sunday walkers   " }
      """
    Then I should receive a 201 JSON response
    And the response field "name" should be "Sunday walkers"

  Scenario: A whitespace-only name is rejected
    When I make a POST call to /api/groups with body
      """
      { "name": "   " }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: An empty name is rejected
    When I make a POST call to /api/groups with body
      """
      { "name": "" }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: A name over 100 characters is rejected
    Given a string of 101 "a" characters is saved as "long_name"
    When I make a POST call to /api/groups with body
      """
      { "name": "{{ long_name }}" }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: A name of exactly 100 characters is accepted
    Given a string of 100 "a" characters is saved as "max_name"
    When I make a POST call to /api/groups with body
      """
      { "name": "{{ max_name }}" }
      """
    Then I should receive a 201 JSON response
    And the response field "name" should be "{{ max_name }}"

  Scenario: A sixth owned group is refused at the cap
    Given "alice" owns 5 groups
    When I make a POST call to /api/groups with body
      """
      { "name": "One too many" }
      """
    Then I should receive a 409 JSON response
    And the response field "code" should be "group_limit_reached"

  Scenario: Being a member of many groups does not block creating one
    Given "alice" is a member of 9 groups
    When I make a POST call to /api/groups with body
      """
      { "name": "Still fine" }
      """
    Then I should receive a 201 JSON response

  Scenario: An anonymous caller cannot create a group
    Given I am anonymous
    When I make a POST call to /api/groups with body
      """
      { "name": "Sunday walkers" }
      """
    Then I should receive a 401 JSON response

  Scenario: The group's join code is never a code that already exists
    Given a group "Existing" owned by "bob" exists with join code "ABCDEFGH"
    When I make a POST call to /api/groups with body
      """
      { "name": "Sunday walkers" }
      """
    Then I should receive a 201 JSON response
    And the response field "joinCode" should match "^[ABCDEFGHJKMNPQRSTVWXYZ23456789]{8}$"
