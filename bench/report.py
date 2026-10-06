"""Compare load benchmark results and compute each runtime's capacity.

Usage: report.py results/go-*.json results/python-*.json [--slo-factor 2] [--max-error-rate 0.01]

Capacity is the highest number of concurrent conversations where
  * the error rate stays at or below --max-error-rate, and
  * conversation p95 stays within --slo-factor times the single-user p95.
Peak throughput is the best completed-conversations-per-second of any level.
"""

import argparse
import json
from pathlib import Path


def capacity(levels: list[dict], slo_factor: float, max_error_rate: float) -> tuple[int | None, float | None]:
    baseline = next((level["conversation_ms"]["p95"] for level in levels if level["conversation_ms"]["p95"]), None)
    if baseline is None:
        return None, None
    best = None
    for level in levels:
        p95 = level["conversation_ms"]["p95"]
        if p95 is None or (level["error_rate"] or 0) > max_error_rate or p95 > slo_factor * baseline:
            break
        best = level["users"]
    return best, slo_factor * baseline


def cpu_ms_per_conversation(level: dict) -> float | None:
    """Worker CPU time per completed conversation: mean worker CPU (percent
    of one core) divided by throughput."""
    cpu = level["services"].get("worker", {}).get("cpu_percent_mean")
    throughput = level["throughput_conv_per_s"]
    if not cpu or not throughput:
        return None
    return round(cpu * 10 / throughput, 1)


def cell(value, suffix: str = "") -> str:
    return "-" if value is None else f"{value}{suffix}"


def label(run: dict) -> str:
    config = run.get("config", {})
    llm = config.get("llm_model") if config.get("llm") == "openrouter" else "fake LLM"
    processes = f", {config['worker_processes']} proc" if run["runtime"] == "python" and config.get("worker_processes") else ""
    access = f", {config['data_access']}" if config.get("data_access") else ""
    return (
        f"{run['runtime']}{access}, {llm}, {config.get('worker_cpus', '?')} CPU{processes}, "
        f"{config.get('worker_replicas', '?')} worker"
    )


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("results", nargs="+")
    parser.add_argument("--slo-factor", type=float, default=2.0)
    parser.add_argument("--max-error-rate", type=float, default=0.01)
    args = parser.parse_args()

    runs = [json.loads(Path(path).read_text()) for path in args.results]
    print("## Summary\n")
    print("| run | capacity (concurrent conversations) | p95 SLO | peak throughput | worker CPU-ms per conversation |")
    print("| --- | --- | --- | --- | --- |")
    for run in runs:
        users, slo = capacity(run["levels"], args.slo_factor, args.max_error_rate)
        peak = max(run["levels"], key=lambda level: level["throughput_conv_per_s"])
        costs = [value for level in run["levels"] if (value := cpu_ms_per_conversation(level)) is not None]
        print(
            f"| {label(run)} | {cell(users)} | {cell(round(slo) if slo else None, ' ms')} "
            f"| {peak['throughput_conv_per_s']} conv/s at {peak['users']} users "
            f"| {cell(min(costs) if costs else None, ' ms')} |"
        )

    for run in runs:
        print(f"\n## {label(run)}\n")
        print("Config: " + ", ".join(f"{key}={value}" for key, value in run.get("config", {}).items()) + "\n")
        print(
            "| users | ok | failed | thr conv/s | conv p50 | conv p95 | conv p99 | server p95 | client gap p95 "
            "| queue wait p95 | job exec p95 | retried | queue max | worker CPU mean/max | worker mem max "
            "| CPU-ms/conv | api CPU max | postgres CPU max | generator CPU max |"
        )
        print("| " + " | ".join(["---"] * 19) + " |")
        for level in run["levels"]:
            services = level["services"]
            worker = services.get("worker", {})
            print(
                f"| {level['users']} | {level['conversations_ok']} | {level['conversations_failed']} "
                f"| {level['throughput_conv_per_s']} "
                f"| {cell(level['conversation_ms']['p50'], ' ms')} | {cell(level['conversation_ms']['p95'], ' ms')} "
                f"| {cell(level['conversation_ms']['p99'], ' ms')} "
                f"| {cell(level.get('server_conversation_ms', {}).get('p95'), ' ms')} "
                f"| {cell(level.get('client_gap_ms', {}).get('p95'), ' ms')} "
                f"| {cell(level['job_queue_wait_ms']['p95'], ' ms')} "
                f"| {cell(level['job_execution_ms']['p95'], ' ms')} | {level['retried_jobs']} "
                f"| {cell(level['queue_ready_max'])} "
                f"| {cell(worker.get('cpu_percent_mean'), '%')} / {cell(worker.get('cpu_percent_max'), '%')} "
                f"| {cell(worker.get('mem_mib_max'), ' MiB')} "
                f"| {cell(cpu_ms_per_conversation(level), ' ms')} "
                f"| {cell(services.get('api', {}).get('cpu_percent_max'), '%')} "
                f"| {cell(services.get('postgres', {}).get('cpu_percent_max'), '%')} "
                f"| {cell(level.get('generator_cpu_percent'), '%')} |"
            )
        errors = {}
        for level in run["levels"]:
            for error, count in level["errors"].items():
                errors[error] = errors.get(error, 0) + count
        if errors:
            print("\nErrors: " + "; ".join(f"{error} x{count}" for error, count in errors.items()))


if __name__ == "__main__":
    main()
