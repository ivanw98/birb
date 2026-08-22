Feature: Leaving a group (POST /api/groups/{id}/leave)

  Idempotent: 204 whether or not the membership was there. An owner cannot leave.

  Background:
    Given a group "Walkers" owned by "bob" exists
    And I am authenticated as "alice"

  Scenario: Leaving a group I joined removes it from my list
    Given "alice" is a member of group "Walkers"
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 204 response
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response body should be
      """
      []
      """

  Scenario: Leaving twice is idempotent
    Given "alice" is a member of group "Walkers"
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 204 response
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 204 response

  Scenario: Leaving a group I was never in is a no-op
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 204 response

  Scenario: Leaving a group that does not exist is a no-op
    When I make a POST call to /api/groups/grp_01j9z3x8k2m4n6p8r0s2t4v6w8/leave
    Then I should receive a 204 response

  Scenario: The owner cannot leave their own group
    Given I am authenticated as "bob"
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 409 JSON response
    And the response field "code" should be "owner_cannot_leave"

  Scenario: The owner's group survives their attempt to leave
    Given I am authenticated as "bob"
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 409 JSON response
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "$" should have 1 items
    And the response field "0.members" should have 1 items

  Scenario: Leaving frees a slot against the membership cap
    Given "alice" is a member of group "Walkers"
    And "alice" is a member of 9 groups
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 204 response
    When I make a POST call to /api/groups with body
      """
      { "name": "Room again" }
      """
    Then I should receive a 201 JSON response

  Scenario: A malformed group id is rejected
    When I make a POST call to /api/groups/not-a-group-id/leave
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: An anonymous caller cannot leave
    Given I am anonymous
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 401 JSON response
