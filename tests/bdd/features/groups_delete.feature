Feature: Deleting a group (DELETE /api/groups/{id})

  Owner only, and the only way a group ends, since ownership never transfers.
  Idempotent. Members' sightings are untouched.

  Background:
    Given a group "Walkers" owned by "bob" exists
    And "alice" is a member of group "Walkers"

  Scenario: The owner deletes the group
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 204 response
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response body should be
      """
      []
      """

  Scenario: Deletion removes the group for every member
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 204 response
    Given I am authenticated as "alice"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response body should be
      """
      []
      """

  Scenario: Deleting twice is idempotent
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 204 response
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 204 response

  Scenario: Deleting a group that does not exist is a no-op
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/grp_01j9z3x8k2m4n6p8r0s2t4v6w8
    Then I should receive a 204 response

  Scenario: A member who is not the owner cannot delete
    Given I am authenticated as "alice"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 403 JSON response
    And the response field "code" should be "not_group_owner"

  Scenario: A stranger cannot delete an existing group
    Given I am authenticated as "carol"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 403 JSON response
    And the response field "code" should be "not_group_owner"

  Scenario: A refused deletion leaves the group intact
    Given I am authenticated as "alice"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 403 JSON response
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "$" should have 1 items

  Scenario: Deleting frees a slot against the owned-groups cap
    Given I am authenticated as "bob"
    And "bob" owns 4 groups
    When I make a POST call to /api/groups with body
      """
      { "name": "Sixth" }
      """
    Then I should receive a 409 JSON response
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 204 response
    When I make a POST call to /api/groups with body
      """
      { "name": "Sixth" }
      """
    Then I should receive a 201 JSON response

  Scenario: Members keep their sightings after the group is deleted
    Given the default user exists
    And the default sighting exists
    And I am authenticated as "bob"
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 204 response
    Given I am authenticated as "default_user"
    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items" should have 1 items

  Scenario: A malformed group id is rejected
    Given I am authenticated as "bob"
    When I make a DELETE call to /api/groups/not-a-group-id
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: An anonymous caller cannot delete
    Given I am anonymous
    When I make a DELETE call to /api/groups/{{group.Walkers.id}}
    Then I should receive a 401 JSON response
