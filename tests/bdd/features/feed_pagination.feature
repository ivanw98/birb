Feature: Paging the feed (GET /api/feed)

  Keyset pagination over (observedAt DESC, sightingId DESC), the same contract as the
  sightings list. A page carries a cursor only when there is more to fetch.

  Background:
    Given a group "Walkers" owned by "bob" exists
    And "alice" is a member of group "Walkers"
    And I am authenticated as "alice"

  Scenario: Newest first
    Given a sighting by "bob" observed 3 hours ago exists as "older"
    And a sighting by "bob" observed 1 hour ago exists as "newer"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.sightingId" should be "{{ sighting.newer.id }}"
    And the response field "items.1.sightingId" should be "{{ sighting.older.id }}"

  Scenario: The last page carries a null cursor
    Given a sighting by "bob" observed 1 hour ago exists as "only"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "nextCursor" should be "null"

  Scenario: A full page carries a cursor
    Given "bob" has 30 sightings from the last week
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 25 items
    And the response field "nextCursor" should not be "null"

  Scenario: Walking the pages covers everything with no overlap
    Given "bob" has 30 sightings from the last week
    And I set query param "limit" to "20"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 20 items
    And I save the response under "page1"
    Given I set query param "limit" to "20"
    And I set query param "cursor" to "{{ page1.nextCursor }}"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 10 items
    And the response field "nextCursor" should be "null"

  Scenario: limit defaults to 25
    Given "bob" has 30 sightings from the last week
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 25 items

  Scenario: limit above the maximum clamps rather than erroring
    Given "bob" has 30 sightings from the last week
    And I set query param "limit" to "500"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 30 items

  Scenario: limit of zero falls back to the default
    Given "bob" has 30 sightings from the last week
    And I set query param "limit" to "0"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items" should have 25 items

  Scenario: A non-integer limit is rejected
    Given I set query param "limit" to "many"
    When I make a GET call to /api/feed
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: A malformed cursor is rejected
    Given I set query param "cursor" to "not-a-cursor"
    When I make a GET call to /api/feed
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: A malformed cursor is rejected even for a caller with no groups
    Given I am authenticated as "carol"
    And I set query param "cursor" to "not-a-cursor"
    When I make a GET call to /api/feed
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"
