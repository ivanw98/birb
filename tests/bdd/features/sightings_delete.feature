Feature: Deleting sightings (tombstones via batch sync)

  Background:
    Given the default user exists
    And I am authenticated as "default_user"

  Scenario: Deleting an existing sighting hides it from the default list
    Given the default sighting exists
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "{{default_sighting.id}}",
            "observedAt": "{{default_sighting.observedAt}}",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T11:00:00Z",
            "deleted": true
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "updated"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response body should be
      """
      { "items": [], "nextCursor": null }
      """

  Scenario: includeDeleted returns the tombstone, marked deleted
    Given the default sighting exists
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "{{default_sighting.id}}",
            "observedAt": "{{default_sighting.observedAt}}",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T11:00:00Z",
            "deleted": true
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "updated"

    When I set query param "includeDeleted" to "true"
    And I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.id" should be "{{default_sighting.id}}"
    And the response field "items.0.deleted" should be "true"

  Scenario: Live rows carry no deleted field, even when tombstones are requested
    Given the default sighting exists
    When I set query param "includeDeleted" to "true"
    And I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.id" should be "{{default_sighting.id}}"
    And the response field "items.0.deleted" should be absent

  Scenario: A newer edit resurrects a deleted sighting
    Given the default sighting exists
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "{{default_sighting.id}}",
            "observedAt": "{{default_sighting.observedAt}}",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T11:00:00Z",
            "deleted": true
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "updated"

    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "{{default_sighting.id}}",
            "observedAt": "{{default_sighting.observedAt}}",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T12:00:00Z",
            "quickNote": "confirmed siskin, un-deleting"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "updated"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.quickNote" should be "confirmed siskin, un-deleting"
    And the response field "items.0.deleted" should be absent

  Scenario: An edit older than the tombstone loses last-write-wins
    Given the default sighting exists
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "{{default_sighting.id}}",
            "observedAt": "{{default_sighting.observedAt}}",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T12:00:00Z",
            "deleted": true
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "updated"

    # An offline device pushes an edit made before it learned of the delete.
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "{{default_sighting.id}}",
            "observedAt": "{{default_sighting.observedAt}}",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T11:00:00Z",
            "quickNote": "edited before the delete happened"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "stale"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response body should be
      """
      { "items": [], "nextCursor": null }
      """

  Scenario: Deleting a sighting the server has never seen creates a tombstone
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_01j9z3x8k2m4n6p8r0s2t4v6w9",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T11:00:00Z",
            "deleted": true
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "created"

    When I make a GET call to /api/sightings
    Then the response body should be
      """
      { "items": [], "nextCursor": null }
      """

    When I set query param "includeDeleted" to "true"
    And I make a GET call to /api/sightings
    Then the response field "items.0.id" should be "sgh_01j9z3x8k2m4n6p8r0s2t4v6w9"
    And the response field "items.0.deleted" should be "true"

  Scenario: A deleted sighting cannot be enriched
    Given the default sighting exists
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "{{default_sighting.id}}",
            "observedAt": "{{default_sighting.observedAt}}",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T11:00:00Z",
            "deleted": true
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "updated"

    When I make a PUT call to /api/sightings/{{default_sighting.id}} with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "notes": "trying to enrich a deleted sighting",
        "photoPaths": []
      }
      """
    Then I should receive a 404 JSON response
    And the response field "code" should be "not_found"

  Scenario: includeDeleted must be a boolean
    When I set query param "includeDeleted" to "banana"
    And I make a GET call to /api/sightings
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"
