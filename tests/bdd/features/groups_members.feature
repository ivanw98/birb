Feature: Removing a member (DELETE /api/groups/{id}/members/{userId})

  Background:
    Given a group "Walkers" owned by "bob" exists
    And "alice" is a member of group "Walkers"

  Scenario: The owner removes a member
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.alice.id}}
    Then I should receive a 204 response
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.members" should have 1 items

  Scenario: The removed member no longer sees the group
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.alice.id}}
    Then I should receive a 204 response
    Given I am authenticated as "alice"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response body should be
      """
      []
      """

  Scenario: Removing twice is idempotent
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.alice.id}}
    Then I should receive a 204 response
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.alice.id}}
    Then I should receive a 204 response

  Scenario: Removing someone who was never a member is a no-op
    Given the user "carol" exists
    And I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.carol.id}}
    Then I should receive a 204 response

  Scenario: Removing from a group that does not exist is a no-op
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/grp_01j9z3x8k2m4n6p8r0s2t4v6w8/members/{{user.alice.id}}
    Then I should receive a 204 response

  Scenario: The owner cannot remove themselves
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.bob.id}}
    Then I should receive a 409 JSON response
    And the response field "code" should be "owner_cannot_leave"

  Scenario: A failed self-removal leaves the owner a member
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.bob.id}}
    Then I should receive a 409 JSON response
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.members" should have 2 items

  Scenario: A member cannot remove another member
    Given the user "carol" exists
    And "carol" is a member of group "Walkers"
    And I am authenticated as "alice"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.carol.id}}
    Then I should receive a 403 JSON response
    And the response field "code" should be "not_group_owner"

  Scenario: A member cannot remove the owner
    Given I am authenticated as "alice"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.bob.id}}
    Then I should receive a 403 JSON response
    And the response field "code" should be "not_group_owner"

  Scenario: A member removing themselves is refused rather than treated as leaving
    Given I am authenticated as "alice"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.alice.id}}
    Then I should receive a 403 JSON response
    And the response field "code" should be "not_group_owner"

  Scenario: A stranger cannot remove anyone
    Given I am authenticated as "carol"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.alice.id}}
    Then I should receive a 403 JSON response
    And the response field "code" should be "not_group_owner"

  Scenario: An evicted member can re-join with the code, which is never rotated
    Given a group "Coded" owned by "bob" exists with join code "BQTX7RKM"
    And "alice" is a member of group "Coded"
    And I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Coded.id}}/members/{{user.alice.id}}
    Then I should receive a 204 response
    Given I am authenticated as "alice"
    When I make a POST call to /api/groups/join with body
      """
      { "code": "BQTX7RKM" }
      """
    Then I should receive a 200 JSON response

  Scenario: A malformed member id is rejected
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/not-a-user-id
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: A malformed group id is rejected
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/not-a-group-id/members/{{user.alice.id}}
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: An anonymous caller cannot remove a member
    Given I am anonymous
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}/members/{{user.alice.id}}
    Then I should receive a 401 JSON response
