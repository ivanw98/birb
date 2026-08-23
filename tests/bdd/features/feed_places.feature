Feature: Nearby place names in the feed (GET /api/feed)

  A sighting resolves to the nearest settlement within 30km.

  Background:
    Given a group "Walkers" owned by "bob" exists
    And "alice" is a member of group "Walkers"
    And I am authenticated as "alice"

  Scenario: A sighting near a place carries that name
    Given a place "Wimbledon Park" exists at 51.4400, -0.2100
    And a sighting by "bob" at 51.4340, -0.2140 observed 2 hours ago exists as "nearby"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.placeName" should be "Wimbledon Park"

  Scenario: The nearer of two places wins
    Given a place "Far Town" exists at 51.6000, -0.2140
    And a place "Near Village" exists at 51.4360, -0.2140
    And a sighting by "bob" at 51.4340, -0.2140 observed 2 hours ago exists as "between"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.placeName" should be "Near Village"

  Scenario: A place beyond 30km is too far to name
    Given a place "Distant" exists at 51.8000, -0.2140
    And a sighting by "bob" at 51.4340, -0.2140 observed 2 hours ago exists as "remote"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.placeName" should be absent

  Scenario: A sighting with no coordinates has no place
    Given a place "Wimbledon Park" exists at 51.4400, -0.2100
    And a sighting by "bob" observed 2 hours ago exists as "nocoords"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.placeName" should be absent

  Scenario: With no places at all, nothing is named
    Given there are no places
    And a sighting by "bob" at 51.4340, -0.2140 observed 2 hours ago exists as "unnamed"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.placeName" should be absent

  Scenario: A place below the population floor is not named
    Given a place "Hamlet" exists at 51.4360, -0.2140 with population 100
    And a sighting by "bob" at 51.4340, -0.2140 observed 2 hours ago exists as "tinyplace"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.placeName" should be absent
