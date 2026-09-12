import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    address: str


def load_settings() -> Settings:
    return Settings(
        address=os.getenv("AGENTS_SERVER_ADDR", "0.0.0.0:8080"),
    )
