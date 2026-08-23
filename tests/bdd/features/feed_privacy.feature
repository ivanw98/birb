Feature: What the feed does not expose (GET /api/feed)

  Membership shares a species, a name, a nearby place and a time.

  Background:
    Given a group "Walkers" owned by "bob" exists
    And "alice" is a member of group "Walkers"
    And a place "Wimbledon Park" exists at 51.4400, -0.2100
    And a sighting by "bob" at 51.4340, -0.2140 observed 2 hours ago exists as "detailed"
    And the sighting "detailed" has notes "nesting site behind the church" and quick note "small brown bird"
    And I am authenticated as "alice"

  Scenario: A feed item leaks neither location, free text, nor identity
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.placeName" should be "Wimbledon Park"
    And the response should not contain "51.434"
    And the response should not contain "-0.214"
    And the response should not contain "nesting site behind the church"
    And the response should not contain "small brown bird"
    And the response should not contain "{{ user.bob.id }}"
    And the response should not contain "bob@example.test"
