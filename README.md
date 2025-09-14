# bsync

A multi-cloud blob storage gateway that issues presigned URLs for **S3**, **Azure Blob Storage**, and **Google Cloud
Storage**. The goal is to provide a unified API surface for developers who need cloud-agnostic access to blob storage
with consistent security and validation.

---

## Project Plan

### v1 Roadmap

**Phase 1: AWS PUT (MVP)**

- Implement `/v1/presign/put` endpoint.
- Support AWS S3 presigner via Lambda entrypoint.

**Phase 2: AWS GET + DELETE — *In Progress***

- Extend AWS integration with `/v1/presign/get` and `/v1/presign/delete`.
- Add integration testing for PUT, GET, DELETE

**Phase 3: Azure Presigner**

- Add Azure Blob Storage support (PUT, GET, DELETE).

**Phase 4: GCP Presigner**

- Add Google Cloud Storage support (PUT, GET, DELETE).

**Phase 5: Azure Entrypoint**

- Deploy Azure presigner via Functions or equivalent.

**Phase 6: GCP Entrypoint**

- Deploy GCP presigner via Cloud Functions or equivalent.

---

## Future Improvements (v2+)

- **Go Client SDK** for interacting with the gateway.
- **Secure Entrypoints**: claims-based authentication & IAM least-privilege.
- **Replication Verification Hooks**: enqueue presign requests for asynchronous validation of object replication across
  providers.
