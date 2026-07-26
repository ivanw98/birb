Feature: Listing sightings (GET /api/sightings)

  Sightings are returned newest-first via keyset pagination on
  (observedAt DESC, id DESC). Soft-deleted rows are excluded (no delete
  endpoint exists in v0.1, so that path is not exercised here).

  Background:
    Given I am authenticated as "alice"

  Scenario: Keyset pagination pages through all sightings with no overlap
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_8xq8fwj515kq3r3sb3e7s8rmw5",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_rs6s2hjnldpgjruys6sjziave7",
            "observedAt": "2025-06-01T11:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T11:00:00Z"
          },
          {
            "id": "sgh_u1zqa0zvhijwuk3tt7f4ucqcaa",
            "observedAt": "2025-06-01T12:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T12:00:00Z"
          },
          {
            "id": "sgh_gu2ue8hyharqjfvxzv6ycm364k",
            "observedAt": "2025-06-01T13:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T13:00:00Z"
          },
          {
            "id": "sgh_wnlm70b9awohazujhhhqa1jcl9",
            "observedAt": "2025-06-01T14:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T14:00:00Z"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response

    Given I set query param "limit" to "2"
    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "nextCursor" should match ".+"
    And the response body should contain
      """
      {
        "items": [
          {
            "id": "sgh_wnlm70b9awohazujhhhqa1jcl9"
          },
          {
            "id": "sgh_gu2ue8hyharqjfvxzv6ycm364k"
          }
        ]
      }
      """
    And I save the response under "page1"

    Given I set query param "limit" to "2"
    And I set query param "cursor" to "{{ page1.nextCursor }}"
    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "nextCursor" should match ".+"
    And the response body should contain
      """
      {
        "items": [
          {
            "id": "sgh_u1zqa0zvhijwuk3tt7f4ucqcaa"
          },
          {
            "id": "sgh_rs6s2hjnldpgjruys6sjziave7"
          }
        ]
      }
      """
    And I save the response under "page2"

    Given I set query param "limit" to "2"
    And I set query param "cursor" to "{{ page2.nextCursor }}"
    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response body should contain
      """
      {
        "items": [
          {
            "id": "sgh_8xq8fwj515kq3r3sb3e7s8rmw5"
          }
        ],
        "nextCursor": null
      }
      """

  Scenario: A user only sees their own sightings
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_3pllgvoji0q4ot52dba2otvrmt",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    Given I am authenticated as "bob"
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_cygmabkbmpicaessfcr7wgth9r",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response body should contain
      """
      {
        "items": [
          {
            "id": "sgh_cygmabkbmpicaessfcr7wgth9r"
          }
        ]
      }
      """

    Given I am authenticated as "alice"
    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response body should contain
      """
      {
        "items": [
          {
            "id": "sgh_3pllgvoji0q4ot52dba2otvrmt"
          }
        ]
      }
      """
