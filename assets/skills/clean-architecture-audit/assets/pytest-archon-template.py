import pytest
from pytest_archon import archrule

def test_clean_architecture_domain_purity():
    """Domain core must not depend on infrastructure, frameworks, or database ORMs."""
    (
        archrule("domain_must_be_pure")
        .match("app.domain.*")
        .should_not_import(
            "app.infrastructure.*",
            "app.adapters.*",
            "app.entrypoints.*",
            "fastapi.*",
            "sqlalchemy.*",
            "httpx.*",
        )
        .check()
    )

def test_application_depends_only_on_domain():
    """Application use cases depend on domain abstractions, not web or DB internals."""
    (
        archrule("application_dependency_inversion")
        .match("app.application.*")
        .should_not_import(
            "app.infrastructure.*",
            "app.entrypoints.*",
            "fastapi.*",
        )
        .check()
    )

def test_no_circular_dependencies():
    """Prevent circular dependency loops between packages."""
    archrule("domain_no_cycles").match("app.domain.*").should_not_import("app.domain.*").check()
