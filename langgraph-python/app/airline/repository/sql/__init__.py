"""asyncpg repositories with hand-written SQL.

A benchmark variant of the SQLAlchemy ORM repositories (selected with
DATA_ACCESS=asyncpg). Each statement in ``queries`` mirrors the sqlc query of
the same name in adk-go/db/query/airline.sql, so both runtimes send the same
SQL to PostgreSQL.
"""
