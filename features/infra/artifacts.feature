# language: en
@infra @migration
Feature: Artifact storage and binary delivery (G2, G6)

  Large files (uploaded .deb, results, binaries) are not inlined into the Dag.
  Object storage (S3) stores them; render.File.Ref{uri, checksum} references them;
  the agent downloads them via a presigned URL. The server delivers the agent+stroppy
  binaries for cloud-init bootstrap (D18) and install-op (D17). Canon: render/system/file.proto
  (File.Ref), internal/infrastructure/s3.

  # G2 — S3 / File.Ref
  Scenario: A large file is stored in S3, not in the Dag
    Given the user uploaded a custom .deb
    When the plan references it
    Then it is a render Item File.Ref{uri: "s3://…", checksum}, not inline content
    And the Dag is not bloated with the file content

  Scenario: The agent downloads an artifact via a presigned URL
    Given File.Ref(s3://…) with a deb_token binding
    When the resolver prepares the command
    Then a presigned GET URL is issued
    And the agent RUN_CMD curl downloads the file (tied to D17 custom .deb)

  Scenario: The bucket is created idempotently
    When the service starts
    Then the bucket is created if absent (CreateS3BucketIfNotExists)

  # G6 — binary delivery
  Scenario: The server delivers the agent binary for cloud-init
    Given a VM comes up with a cloud-init binaryURL
    When the VM downloads the binary
    Then the server delivers the agent binary (D18 bootstrap)

  Scenario: The stroppy binary is resolved by version
    Given Workload.stroppy_version = "5.1.1"
    When stroppy install is planned
    Then the binary is taken from cache or downloaded (github release v5.1.1)
    And the install-op gets the path/URL (D17)

  # --- error / edge cases ---

  Scenario: A failed artifact download fails the node and is retryable
    Given the agent RUN_CMD curl of a presigned URL fails
    Then the install/WRITE_FILE node fails and is retried by node retry (engine)

  Scenario: A checksum mismatch is rejected
    Given File.Ref{uri, checksum} whose downloaded bytes do not match checksum
    Then the artifact is rejected and the node fails

  Scenario: An expired presigned URL is re-issued
    Given a presigned GET URL that expired before download
    When the resolver prepares the command again
    Then a fresh presigned URL is issued

  Scenario: A binary cache miss downloads once and reuses
    Given a stroppy version not in the cache
    When two installs need it
    Then it is downloaded once and served from cache afterwards
