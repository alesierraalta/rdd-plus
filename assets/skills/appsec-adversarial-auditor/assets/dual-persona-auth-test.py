import pytest
import requests

BASE_URL = "http://localhost:8000/api/v1"

@pytest.fixture
def persona_alice():
    """Legitimate owner of workspace and resources."""
    return {"token": "Bearer token_alice", "tenant_id": "tenant_a"}

@pytest.fixture
def persona_mallory():
    """Attacker attempting cross-tenant access."""
    return {"token": "Bearer token_mallory", "tenant_id": "tenant_b"}

def test_bola_cross_tenant_isolation(persona_alice, persona_mallory):
    # 1. Alice creates a private document
    create_resp = requests.post(
        f"{BASE_URL}/documents",
        json={"title": "Confidential Roadmaps", "content": "Top Secret"},
        headers={"Authorization": persona_alice["token"]}
    )
    assert create_resp.status_code == 201
    doc_id = create_resp.json()["id"]

    # 2. Mallory attempts to read Alice's document via direct object reference
    read_resp = requests.get(
        f"{BASE_URL}/documents/{doc_id}",
        headers={"Authorization": persona_mallory["token"]}
    )
    assert read_resp.status_code in [403, 404], (
        f"CRITICAL BOLA VULNERABILITY: Mallory accessed Alice's document {doc_id}!"
    )

    # 3. Mallory attempts to overwrite Alice's document
    mutate_resp = requests.put(
        f"{BASE_URL}/documents/{doc_id}",
        json={"title": "Hacked Title"},
        headers={"Authorization": persona_mallory["token"]}
    )
    assert mutate_resp.status_code in [403, 404], (
        f"CRITICAL BOLA VULNERABILITY: Mallory modified Alice's document {doc_id}!"
    )

    # 4. Mallory attempts to delete Alice's document
    delete_resp = requests.delete(
        f"{BASE_URL}/documents/{doc_id}",
        headers={"Authorization": persona_mallory["token"]}
    )
    assert delete_resp.status_code in [403, 404], (
        f"CRITICAL BOLA VULNERABILITY: Mallory deleted Alice's document {doc_id}!"
    )
