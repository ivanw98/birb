Feature: Listing groups (GET /api/groups)

  Every group the caller belongs to, owned or joined, with full membership.

  Background:
    Given I am authenticated as "alice"

  Scenario: A caller in no groups gets an empty array
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response body should be
      """
      []
      """

  Scenario: Owned and joined groups are returned together
    Given a group "Mine" owned by "alice" exists
    And a group "Theirs" owned by "bob" exists
    And "alice" is a member of group "Theirs"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "$" should have 2 items

  Scenario: isOwner distinguishes a group I own from one I joined
    Given a group "Mine" owned by "alice" exists
    And a group "Theirs" owned by "bob" exists
    And "alice" is a member of group "Theirs"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.name" should be "Mine"
    And the response field "0.isOwner" should be "true"
    And the response field "1.name" should be "Theirs"
    And the response field "1.isOwner" should be "false"

  Scenario: The owner is listed first among members
    Given a group "Theirs" owned by "bob" exists
    And "alice" is a member of group "Theirs"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.members" should have 2 items
    And the response field "0.members.0.id" should be "{{ user.bob.id }}"
    And the response field "0.members.0.isOwner" should be "true"
    And the response field "0.members.1.isOwner" should be "false"

  Scenario: A member with no display name omits the name field
    Given a group "Theirs" owned by "bob" exists
    And "alice" is a member of group "Theirs"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.members.0.name" should be absent

  Scenario: A member's display name is returned when set
    Given the user "bob" exists with display name "Bob"
    And a group "Theirs" owned by "bob" exists
    And "alice" is a member of group "Theirs"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.members.0.name" should be "Bob"

  Scenario: A group I am not in is never listed
    Given a group "Strangers" owned by "bob" exists
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response body should be
      """
      []
      """

  Scenario: The join code is returned to members
    Given a group "Theirs" owned by "bob" exists with join code "ABCDEFGH"
    And "alice" is a member of group "Theirs"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.joinCode" should be "ABCDEFGH"

  Scenario: Member email addresses are never exposed
    Given the user "bob" exists with display name "Bob"
    And a group "Theirs" owned by "bob" exists
    And "alice" is a member of group "Theirs"
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response should not contain "bob@example.test"
    And the response should not contain "alice@example.test"

  Scenario: A full group returns every member
    Given a group "Big" owned by "alice" exists
    And group "Big" has 24 other members
    When I make a GET call to /api/groups
    Then I should receive a 200 JSON response
    And the response field "0.members" should have 25 items

  Scenario: An anonymous caller cannot list groups
    Given I am anonymous
    When I make a GET call to /api/groups
    Then I should receive a 401 JSON response
