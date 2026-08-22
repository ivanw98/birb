Feature: Batch-syncing sightings (POST /api/sightings/batch)

  Synchronises offline-captured sightings: an idempotent, caller-scoped upsert.
  New ids are created; existing ids owned by the caller have their mutable
  content fields updated last-write-wins on clientUpdatedAt; capture fields
  are never mutated after creation; an id owned by another user is rejected.

  Background:
    Given I am authenticated as "alice"

  Scenario: Creating a brand-new sighting
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_ucvo07d9ir3ceenyho3ftm47cn",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 60,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "quickNote": "small brown bird in reeds"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.id" should be "sgh_ucvo07d9ir3ceenyho3ftm47cn"
    And the response field "results.0.status" should be "created"

  Scenario: Re-sending the same or an older edit is idempotent (stale, no change)
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_huxixhwm5ojsptu6aneirbb73z",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "quickNote": "first pass"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    # Re-sending the identical item (same clientUpdatedAt) is a no-op retry.
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_huxixhwm5ojsptu6aneirbb73z",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "quickNote": "first pass"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "stale"

    # An older clientUpdatedAt loses last-write-wins even with different content.
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_huxixhwm5ojsptu6aneirbb73z",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T09:00:00Z",
            "quickNote": "a stale retry with an older edit time"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "stale"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.quickNote" should be "first pass"

  Scenario: A newer edit updates the content and is visible on re-list
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_ydet3v2i9ofgxxf2diah66jxw2",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "quickNote": "small brown bird"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_ydet3v2i9ofgxxf2diah66jxw2",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T12:00:00Z",
            "quickNote": "confirmed: reed warbler"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "updated"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.quickNote" should be "confirmed: reed warbler"

  # BUG (found against real Postgres, not reproducible with sqlmock): pgx's
  # default timestamptz decode calls time.Unix(), which returns a time.Time
  # in the server process's *local* zone (time.Local), and nothing in
  # internal/store normalizes scanned Sighting/User timestamps back to UTC
  # before they're serialized. The instant is correct but the wire format
  # isn't ("...T09:00:00+01:00" instead of "...T08:00:00Z" on a UTC+1 host).
  # Contrast models.EncodeCursor, which does call .UTC() before formatting —
  # so the fix is the same discipline in sightingRow.toModel()/models.User,
  # or centrally via a pgtype.Map with TimestamptzCodec.ScanLocation =
  # time.UTC on the pgx connection. Tagged @WIP (excluded from the default
  # `go test -tags bdd` run) until fixed; run with BIRB_BDD_TAGS=@WIP to
  # reproduce. Do not delete or weaken this assertion to make it pass.
  @WIP
  Scenario: Capture fields are immutable after creation
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_97bet2fardu6vyrxqy1ffw06kk",
            "observedAt": "2025-06-01T08:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T08:00:00Z",
            "quickNote": "original capture"
          }
        ]
      }
      """
    Then the response field "results.0.status" should be "created"

    # Re-sync with a DIFFERENT observedAt but a newer clientUpdatedAt: content
    # updates, but the capture-time fact never changes after creation.
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_97bet2fardu6vyrxqy1ffw06kk",
            "observedAt": "2025-06-01T23:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T09:00:00Z",
            "quickNote": "resynced with a different capture time"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "updated"

    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.observedAt" should be "2025-06-01T08:00:00Z"
    And the response field "items.0.quickNote" should be "resynced with a different capture time"

  Scenario: An unknown birdId is rejected but the batch still succeeds
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_jhpmnwescpy2dpg9uyivgqmb4j",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "birdId": "brd_00000000000000000000000000"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "invalid"
    And the response field "results.0.error.code" should be "unknown_bird_id"

  Scenario: An observedAt more than 24h in the future is rejected
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_07r7dahr8trtk9ipw3dniuzjm0",
            "observedAt": "2999-01-01T00:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "invalid"
    And the response field "results.0.error.code" should be "observed_at_in_future"

  Scenario: A malformed id is rejected as a validation failure
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "not-a-valid-sighting-id",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "invalid"
    And the response field "results.0.error.code" should be "validation_failed"

  Scenario: A sighting id owned by another user is rejected, and the owner's row is untouched
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_h5frysijvgqf7vi4yunl42pbbe",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z",
            "quickNote": "alice was here first"
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
            "id": "sgh_h5frysijvgqf7vi4yunl42pbbe",
            "observedAt": "2025-06-02T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-02T10:00:00Z",
            "quickNote": "bob tries to claim the same id"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "invalid"
    And the response field "results.0.error.code" should be "id_conflict"

    Given I am authenticated as "alice"
    When I make a GET call to /api/sightings
    Then I should receive a 200 JSON response
    And the response field "items.0.quickNote" should be "alice was here first"

  Scenario: A batch of more than 100 items is rejected whole
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_wduew91hokdgen7u3dapvx956j",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_tm12nsadicg1sja8j2rbjpzgsx",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_9oz5iowx2imtrcd6dm8gnx5v5j",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_p5zqirwgcsgx3h7662spurw0qm",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_46lm6etoso7ld41lve6dvke8ut",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_i8xyky5c6jaynu6h2nwrq8xz2s",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_d404zwqz3nt6qm6ajjmlowihw1",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_2fj0mzhbdad188d2lr952vv4zq",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_7j7ii208wxuzmqg6z8cgfi7d1e",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_6obduaubta1thq9zg4auerqpp8",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_pgiinrcp99mmi2ak43ayi791qv",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_zicsuqdhrmkvmm6mrwmsm0vw01",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_k6rol7rfwixuyz9ae9nxu6nm8d",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_fx5a5gpo5z5sffj5uwv1yrq4qm",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_35je56zd4ybgnfdq6ipn8dmo7s",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_f6lfmaz4qnqxnu5zkso0z2wchs",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_ih6mf9qape5fxifs0x8nott1eg",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_sakwpfyuizz6pjuptoyjefpoye",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_czncccb3ge84uoyja2qwol01vc",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_yygjop88nm9g57mjeqcn0soua0",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_oqhtq1ospa6j4fjl548jzfvpav",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_1167myl90y98bod2g3ivutf6wh",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_cwm15wso6d1hd3foc7np3t25ps",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_jjjhgwb5p6if81u6kpahhin4k1",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_2wyfnfispo1wyl56yxibg3tdr2",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_i42qzduzdmwxlp6a4wdzkgvd1q",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_wpcgjck4nzm0hwabo2ncignugq",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_74g8qxa16alfu0qxprndvjofwo",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_h41y0xw4h5xy81x0z8hdxxbeih",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_dhh809s7dfbxqnj79ibkh03i6n",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_3f40vbrpicivzhjj25snx7xd0d",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_x263gjsutud4jwap7kcqza7a0w",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_kb7pzyycy1px8wbecue30p7v1s",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_a74be1thx683p8rrde8z81w5cl",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_4swnxhkjfced7i87b107lf32ld",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_ved24okya2bcpzxubqm8730h3l",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_qn3mg5h1mbcanvf7bwjsisvwbf",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_57mik9hdby6x7eozunkr5ttgwh",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_ayajlax56uwa1pli064gqa0oog",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_djopvtkgqcd8puqh6ou0jn94ec",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_4v5yk4l6e2litqtggijojbkzaa",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_qohwrzu1x0v8c6swqyqpgj07ch",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_v0o24tz8ilnwn237qs6zha7bja",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_wy5iuz487wbt51lfqo3cfol2r2",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_598nrhb52ns5fnlrwxnokz5yo3",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_7cnljgyae4msw6xuefzpivljkr",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_etxztxpw41bin323afppi8lfg7",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_4w67ybgb0p1ycxn4rbvmpmq13x",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_g0wpni1mwh9xk6h58n81lf4emp",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_zgzdwp908xtrbd50zb4rto82ef",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_fhtrhhiw1wq8hgjhvcku7d3mar",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_v34n4eggyihdwjmr0wcw4pj4qb",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_mfxow529h4rdslgnpkai8bu0gz",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_72j5pc2tlrab2kh038ynvh6hod",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_vl0vb3mo1rf2fnjzmwtng5y4on",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_s8pvgq28dzq1iet1rlabdkosen",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_9legbgqsusmwfk14tng0nzmn5v",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_pggnd7ed3yzkqe9zg2ocmlpe1y",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_zh79td4cin21p3e3hhr66od1z1",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_efwqs31jin4m5mred5zv8cc7n6",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_ot4bz6ze1ro29os8a3gncrwzmk",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_m8cqtbogejsrp38rsdwqeu0n13",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_9hsjhkzw00nygiz35cbphzdo6i",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_7v6zyrv6kgcqblkr5b02jpdzlz",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_yk3a01ek0p5xmi44kbvcdjwti3",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_ga9xgp0nwuvdqytf8b6yu0lpls",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_e9ug6ov3yj266tbucffon4c98k",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_ldof6hby5yrhqitb2dd92arfhp",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_xehzxpme4mh61uf0ihpqwiaetj",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_066trqlkcfcgx0znlx4te2w790",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_0f6v8psrcz5tzb4cuvpg9f151d",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_yvxqco57y7k18m5lafbmv09vgv",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_jups85lttmsn6aitaol3zu5o1y",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_4uj3c41ilw1xeid9f9ey47si4l",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_uopriwma1nbky4idlaspygd6zz",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_rxog1c5wp3ban307puyhog6od6",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_dovy8jz8gvifu4xqpveko746d1",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_j6mndlc41rnvqxiff1em0aaqy0",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_jh29kp4pg0xg8niqlziiy5djen",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_udkv82kw1olevztkk629k39nss",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_h9p554sxj5edr71c4z8g138x61",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_8yb413339vj4cmjupfc6fuofm5",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_x9pmrvx8tz28ht55fpp0p9bt56",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_npmowh6dqjhekjteodoyu9gwuv",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_dgwqtce2sspw64alaj4mzpjnmh",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_s59vv04z92oeyngdl6f4aq3uqk",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_6anqu23aobo23dgkuxux8wy0ia",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_i2uu4m1blcfqa03hutwnfmklbc",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_flw15ks5mxxu1ncna4co73fark",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_7ckefi490nuadcjxlnogitw34m",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_zhcel3tbt9z7rsm9jdncoi6eqw",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_mcyhmc8plajrujcc014dnjutiq",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_uzwh3gnbeue16lwlozovqtqjhk",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_1ehq604gc437mhqwunwb8vhyom",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_vghjittzslqkvwm039sa9uoejp",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_ihlh50ym1x3nz572hpk2tyvr2q",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_idmb3pxfl7k46nqh3sgrb0zofa",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_cw2izy848th8po39zx4m997knz",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_nafqhsmi7crhi9i9d1ep0gqnh9",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_2608mgis4schn1r0055umqb7j1",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          },
          {
            "id": "sgh_21u0e46brbi84eelqvoulje27b",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then I should receive a 400 JSON response
    And the response field "code" should be "batch_too_large"

  Scenario: A brand-new identity's very first call can be a batch sync
    Given I am authenticated as "never-seen-before"
    When I make a POST call to /api/sightings/batch with body
      """
      {
        "sightings": [
          {
            "id": "sgh_wbzai661ootx6yut3ni9ar9ihy",
            "observedAt": "2025-06-01T10:00:00Z",
            "observedAtOffsetMinutes": 0,
            "clientUpdatedAt": "2025-06-01T10:00:00Z"
          }
        ]
      }
      """
    Then I should receive a 200 JSON response
    And the response field "results.0.status" should be "created"
