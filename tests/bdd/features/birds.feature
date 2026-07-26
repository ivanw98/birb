Feature: Bird reference list (GET /api/birds)

  The list is seeded once by db/migrations/00002_seed_birds.sql and changes
  approximately never, so clients cache it and revalidate with If-None-Match.

  Background:
    Given I am authenticated as "birder"

  Scenario: Listing birds returns the full seeded reference list with a strong ETag
    When I make a GET call to /api/birds
    Then I should receive a 200 JSON response
    And the response header "ETag" should not be empty
    And the response field "0.id" should match "^brd_[a-z0-9]{26}$"

  Scenario: Revalidating with the current ETag returns 304 with no change
    When I make a GET call to /api/birds
    Then I should receive a 200 JSON response
    And I save the response header "ETag" as "etag"

    Given I set header "If-None-Match" to "{{ etag }}"
    When I make a GET call to /api/birds
    Then I should receive a 304 response

  Scenario: A stale If-None-Match does not match and the full list is returned
    Given I set header "If-None-Match" to "deadbeef"
    When I make a GET call to /api/birds
    Then I should receive a 200 JSON response
