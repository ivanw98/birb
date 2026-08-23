Feature: The friend feed (GET /api/feed)

  Co-members' sightings from the last seven days, newest first. (Not the caller's own,
  not a stranger's, and not a tombstone deleted sighting) Sharing two groups with someone shows their
  sighting once, not twice.

  Background:
    Given a group "Walkers" owned by "bob" exists
    And "alice" is a member of group "Walkers"
    And I am authenticated as "alice"

  Scenario: A caller in no groups gets an empty page
    Given I am authenticated as "carol"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 0 items
    And the response field "nextCursor" should be "null"

  Scenario: A group whose only member is the owner yields an empty feed
    Given a group "Solo" owned by "carol" exists
    And I am authenticated as "carol"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 0 items

  Scenario: A co-member's sighting appears
    Given a sighting by "bob" observed 2 hours ago exists as "recent"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 1 items
    And the response field "items.0.sightingId" should be "{{ sighting.recent.id }}"

  Scenario: My own sightings never appear in my feed
    Given a sighting by "alice" observed 2 hours ago exists as "mine"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 0 items

  Scenario: Someone who shares no group with me never appears
    Given a sighting by "dave" observed 2 hours ago exists as "strangers"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 0 items

  Scenario: A sighting six days old is inside the window
    Given a sighting by "bob" observed 6 days ago exists as "justinside"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 1 items

  Scenario: A sighting eight days old is outside the window
    Given a sighting by "bob" observed 8 days ago exists as "tooold"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 0 items

  Scenario: A future-dated sighting cannot pin itself to the top
    Given a sighting by "bob" observed 2 hours ago exists as "recent"
    And a sighting by "bob" observed 12 hours from now exists as "future"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 1 items
    And the response field "items.0.sightingId" should be "{{ sighting.recent.id }}"

  Scenario: A soft-deleted sighting disappears from the feed
    Given a sighting by "bob" observed 2 hours ago exists as "doomed"
    And the sighting "doomed" is soft deleted
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 0 items

  Scenario: A friend shared through two groups appears once
    Given a group "Second" owned by "bob" exists
    And "alice" is a member of group "Second"
    And a sighting by "bob" observed 2 hours ago exists as "shared"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 1 items

  Scenario: The author's display name is returned when set
    Given I am authenticated as "bob" with display name "Bob"
    When I make a GET call to /api/me
    Given I am authenticated as "alice"
    And a sighting by "bob" observed 2 hours ago exists as "named"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.authorName" should be "Bob"

  Scenario: An author with no display name omits the field
    Given a sighting by "bob" observed 2 hours ago exists as "anon"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.authorName" should be absent

  Scenario: A sighting with no species omits birdId
    Given a sighting by "bob" observed 2 hours ago exists as "unidentified"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.birdId" should be absent

  Scenario: Leaving a group drops my sightings from the remaining member's feed
    Given a sighting by "bob" observed 2 hours ago exists as "leaving"
    And I am authenticated as "bob"
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 409 JSON response
    Given I am authenticated as "alice"
    When I make a POST call to /api/groups/{{group.Walkers.id}}/leave
    Then I should receive a 204 response
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 0 items

  Scenario: An anonymous caller cannot read the feed
    Given I am anonymous
    When I make a GET call to /api/feed
    Then I should receive a 401 JSON response
