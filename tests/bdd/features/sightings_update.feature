Feature: Updating a sighting (PUT /api/sightings/{id})

  Enriches an existing sighting: a full replace of the mutable content fields.
  Capture facts (observedAt, offset, coordinates) are immutable and are not
  part of this request. A stale write (clientUpdatedAt older than stored)
  returns 409 with the current server state so an interactive UI can
  reconcile; an unknown or foreign id returns 404.

  Background:
    Given I am authenticated as "alice"
    And a seeded bird is saved as "birdId"

  Scenario: Enriching an existing sighting replaces its content fields
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_ddmqs8hdce2k8dub7ci6tm1yzm",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "quickNote": "small brown bird"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    # PUT is a full replace of content fields (per SightingUpdate: a field
    # omitted from the request is cleared), so a realistic enrichment client
    # reads the current state and carries quickNote forward alongside the
    # new birdId/notes rather than expecting it preserved for free.
    When I make a PUT call to /api/sightings/sgh_ddmqs8hdce2k8dub7ci6tm1yzm with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "birdId": "{{ birdId }}",
        "quickNote": "small brown bird",
        "notes": "confirmed after checking the field guide",
        "photoPaths": []
      }
      """
    Then I should receive a 200 JSON response
    And the response field "birdId" should be "{{ birdId }}"
    And the response field "notes" should be "confirmed after checking the field guide"
    And the response field "quickNote" should be "small brown bird"

  Scenario: A stale write is rejected with the current server state
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_plqu8eeve9gh1ap48ezqkv94m5",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T12:00:00Z",
            "quickNote": "first version"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a PUT call to /api/sightings/sgh_plqu8eeve9gh1ap48ezqkv94m5 with body
      """
      {
        "clientUpdatedAt": "2025-06-01T09:00:00Z",
        "quickNote": "an edit that arrives late",
        "photoPaths": []
      }
      """
    Then I should receive a 409 JSON response
    And the response field "code" should be "stale_update"
    And the response field "current.id" should be "sgh_plqu8eeve9gh1ap48ezqkv94m5"
    And the response field "current.quickNote" should be "first version"

  Scenario: Updating a nonexistent sighting returns 404
    When I make a PUT call to /api/sightings/sgh_00000000000000000000000000 with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": []
      }
      """
    Then I should receive a 404 response
    And the response field "code" should be "not_found"

  Scenario: Updating another user's sighting returns 404, not a leak
    Given I am authenticated as "bob"
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_vjljbtswlohwce6lfyosjexpsj",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    Given I am authenticated as "alice"
    When I make a PUT call to /api/sightings/sgh_vjljbtswlohwce6lfyosjexpsj with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": []
      }
      """
    Then I should receive a 404 response

  Scenario: Photo paths owned by the caller round-trip through Postgres text[]
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_hhi52uerahz6v1epimvcne8vgl",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a PUT call to /api/sightings/sgh_hhi52uerahz6v1epimvcne8vgl with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": ["{{ current_user.auth_id }}/sgh_hhi52uerahz6v1epimvcne8vgl/a.jpg", "{{ current_user.auth_id }}/sgh_hhi52uerahz6v1epimvcne8vgl/b.png"]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "photoPaths.0" should be "{{ current_user.auth_id }}/sgh_hhi52uerahz6v1epimvcne8vgl/a.jpg"
    And the response field "photoPaths.1" should be "{{ current_user.auth_id }}/sgh_hhi52uerahz6v1epimvcne8vgl/b.png"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.photoPaths.0" should be "{{ current_user.auth_id }}/sgh_hhi52uerahz6v1epimvcne8vgl/a.jpg"
    And the response field "items.0.photoPaths.1" should be "{{ current_user.auth_id }}/sgh_hhi52uerahz6v1epimvcne8vgl/b.png"

  Scenario: A photo path not owned by the caller is rejected
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_4k351zijvsq786cz69tk6m4v03",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a PUT call to /api/sightings/sgh_4k351zijvsq786cz69tk6m4v03 with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": ["someone-elses-uid/sgh_4k351zijvsq786cz69tk6m4v03/a.jpg"]
      }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "invalid_photo_path"
