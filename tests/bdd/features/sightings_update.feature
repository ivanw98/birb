Feature: Updating a sighting (PUT /api/sightings/{id})

  Enriches an existing sighting: a full replace of the mutable content fields.

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

  Scenario: A clientUpdatedAt far in the future is rejected, protecting last-write-wins
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_futureclock000000000000000",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "quickNote": "honest capture"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    # If this write landed, its timestamp would outrank every later edit of the
    # row forever (both batch and PUT would compare stale against year 2999).
    When I make a PUT call to /api/sightings/sgh_futureclock000000000000000 with body
      """
      {
        "clientUpdatedAt": "2999-01-01T00:00:00Z",
        "quickNote": "poisoned clock",
        "photoPaths": []
      }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "client_updated_at_in_future"

    # The row is untouched and still editable by an honest timestamp.
    When I make a PUT call to /api/sightings/sgh_futureclock000000000000000 with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "quickNote": "still editable",
        "photoPaths": []
      }
      """
    Then I should receive a 200 JSON response
    And the response field "quickNote" should be "still editable"

  Scenario: Overlong notes are rejected as validation_failed, not a server error
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_longnotes00000000000000000",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    # 5001 chars exceeds the 5000-char notes limit; the service must reject it
    # as a 400 before the DB CHECK constraint can turn it into a 500.
    Given a string of 5001 "x" characters is saved as "blob"
    When I make a PUT call to /api/sightings/sgh_longnotes00000000000000000 with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "notes": "{{ blob }}",
        "photoPaths": []
      }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"

  Scenario: Recording paths owned by the caller round-trip through Postgres text[]
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_rec1auerahz6v1epimvcne8vgl",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a PUT call to /api/sightings/sgh_rec1auerahz6v1epimvcne8vgl with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": [],
        "recordingPaths": ["{{ current_user.auth_id }}/sgh_rec1auerahz6v1epimvcne8vgl/a.webm", "{{ current_user.auth_id }}/sgh_rec1auerahz6v1epimvcne8vgl/b.m4a"]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "recordingPaths.0" should be "{{ current_user.auth_id }}/sgh_rec1auerahz6v1epimvcne8vgl/a.webm"
    And the response field "recordingPaths.1" should be "{{ current_user.auth_id }}/sgh_rec1auerahz6v1epimvcne8vgl/b.m4a"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.recordingPaths.0" should be "{{ current_user.auth_id }}/sgh_rec1auerahz6v1epimvcne8vgl/a.webm"
    And the response field "items.0.recordingPaths.1" should be "{{ current_user.auth_id }}/sgh_rec1auerahz6v1epimvcne8vgl/b.m4a"

  Scenario: A recording path not owned by the caller is rejected
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_rec2auerahz6v1epimvcne8vgl",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a PUT call to /api/sightings/sgh_rec2auerahz6v1epimvcne8vgl with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": [],
        "recordingPaths": ["someone-elses-uid/sgh_rec2auerahz6v1epimvcne8vgl/a.webm"]
      }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "invalid_recording_path"

  Scenario: A recording path with a codec MediaRecorder never produces is rejected
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_rec3auerahz6v1epimvcne8vgl",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a PUT call to /api/sightings/sgh_rec3auerahz6v1epimvcne8vgl with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": [],
        "recordingPaths": ["{{ current_user.auth_id }}/sgh_rec3auerahz6v1epimvcne8vgl/a.mp3"]
      }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "invalid_recording_path"

  Scenario: More than five recording paths is rejected
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_rec4auerahz6v1epimvcne8vgl",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a PUT call to /api/sightings/sgh_rec4auerahz6v1epimvcne8vgl with body
      """
      {
        "clientUpdatedAt": "2025-06-01T12:00:00Z",
        "photoPaths": [],
        "recordingPaths": [
          "{{ current_user.auth_id }}/sgh_rec4auerahz6v1epimvcne8vgl/a.webm",
          "{{ current_user.auth_id }}/sgh_rec4auerahz6v1epimvcne8vgl/b.webm",
          "{{ current_user.auth_id }}/sgh_rec4auerahz6v1epimvcne8vgl/c.webm",
          "{{ current_user.auth_id }}/sgh_rec4auerahz6v1epimvcne8vgl/d.webm",
          "{{ current_user.auth_id }}/sgh_rec4auerahz6v1epimvcne8vgl/e.webm",
          "{{ current_user.auth_id }}/sgh_rec4auerahz6v1epimvcne8vgl/f.webm"
        ]
      }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "validation_failed"
