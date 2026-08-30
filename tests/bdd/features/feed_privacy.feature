Feature: What the feed does not expose (GET /api/feed)

  Membership shares a species, a name, a nearby place, a time, and any attached photos or
  recordings. It never shares exact location, free text, or the sighter's identity.

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

  Scenario: Attached media is shared, disclosing the author's storage-path auth id, but nothing else
    Given the sighting "detailed" has photo "{{ user.bob.authId }}/{{ sighting.detailed.id }}/a.jpg" and recording "{{ user.bob.authId }}/{{ sighting.detailed.id }}/a.webm"
    When I make a GET call to /api/feed
    Then I should receive a 200 JSON response
    And the response field "items.0.photoPaths.0" should be "{{ user.bob.authId }}/{{ sighting.detailed.id }}/a.jpg"
    And the response field "items.0.recordingPaths.0" should be "{{ user.bob.authId }}/{{ sighting.detailed.id }}/a.webm"
    And the response should not contain "51.434"
    And the response should not contain "-0.214"
    And the response should not contain "nesting site behind the church"
    And the response should not contain "small brown bird"
    And the response should not contain "{{ user.bob.id }}"
    And the response should not contain "bob@example.test"
