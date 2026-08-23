Feature: What the feed does not expose (GET /api/feed)

  Membership shares a species, a name, a nearby place and a time. It does not share where
  someone was, what they wrote, or who they are beyond a display name. These scenarios
  assert against the raw body, so a field leaking under a name nobody thought to check
  still fails.

  Background:
    Given a group "Walkers" owned by "bob" exists
    And "alice" is a member of group "Walkers"
    And a place "Wimbledon Park" exists at 51.4400, -0.2100
    And a sighting by "bob" at 51.4340, -0.2140 observed 2 hours ago exists as "detailed"
    And the sighting "detailed" has notes "nesting site behind the church" and quick note "small brown bird"
    And I am authenticated as "alice"

  Scenario: A feed item carries only the five contracted fields
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 1 items
    And the response field "items.0.sightingId" should be "{{ sighting.detailed.id }}"
    And the response field "items.0.observedAt" should not be "null"
    And the response field "items.0.placeName" should be "Wimbledon Park"

  Scenario: Coordinates never reach the caller
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.latitude" should be absent
    And the response field "items.0.longitude" should be absent
    And the response should not contain "51.434"
    And the response should not contain "-0.214"

  Scenario: Free text never reaches the caller
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.notes" should be absent
    And the response field "items.0.quickNote" should be absent
    And the response should not contain "nesting site behind the church"
    And the response should not contain "small brown bird"

  Scenario: The author's identity is a display name and nothing more
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.userId" should be absent
    And the response field "items.0.authorId" should be absent
    And the response should not contain "{{ user.bob.id }}"
    And the response should not contain "bob@example.test"

  Scenario: Photo paths are not part of the feed
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.photoPaths" should be absent

  Scenario: Sync metadata is not part of the feed
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.clientUpdatedAt" should be absent
    And the response field "items.0.createdAt" should be absent
    And the response field "items.0.updatedAt" should be absent
    And the response field "items.0.deleted" should be absent
