import os
import re
import time
import json
import requests
import psycopg2
import docker
from datetime import datetime, timezone
from zoneinfo import ZoneInfo

TOKEN = os.environ.get("TELEGRAM_BOT_TOKEN", "").strip()
API_BASE_URL = os.environ.get("API_BASE_URL", "http://go_api:3001").rstrip("/")
POLL_SECONDS = int(os.environ.get("TELEGRAM_POLL_SECONDS", "3"))
WATCH_SECONDS = int(os.environ.get("TELEGRAM_WATCH_SECONDS", "20"))
TELEGRAM_TIMEZONE = os.environ.get("TELEGRAM_TIMEZONE", "America/Sao_Paulo")
TELEGRAM_WATCH_NOTIFY_EVENTS = os.environ.get("TELEGRAM_WATCH_NOTIFY_EVENTS", "false").strip().lower() in ("1", "true", "yes", "on")

POSTGRES_HOST = os.environ.get("POSTGRES_HOST", "postgres")
POSTGRES_PORT = int(os.environ.get("POSTGRES_PORT", "5432"))
POSTGRES_DB = os.environ.get("POSTGRES_DB", "iiot_platform")
POSTGRES_USER = os.environ.get("POSTGRES_USER", "admin")
POSTGRES_PASSWORD = os.environ.get("POSTGRES_PASSWORD", "")

ALLOWED_CHAT_IDS = set()
raw_allowed = os.environ.get("TELEGRAM_ALLOWED_CHAT_IDS", "")
for item in raw_allowed.split(","):
    item = item.strip()
    if item:
        ALLOWED_CHAT_IDS.add(item)

if not TOKEN:
    raise RuntimeError("TELEGRAM_BOT_TOKEN is required")

BASE = f"https://api.telegram.org/bot{TOKEN}"

docker_client = docker.from_env()


def send_message(chat_id: str, text: str):
    requests.post(
        f"{BASE}/sendMessage",
        data={"chat_id": chat_id, "text": text, "disable_web_page_preview": True},
        timeout=10,
    )


def render_event(title: str, fields: dict[str, str]):
    order = [
        "email",
        "role",
        "user_id",
        "tenant_id",
        "user_email",
        "device_id",
        "device_label",
        "status",
        "source",
        "request_id",
        "horario",
    ]
    lines = [title]
    for key in order:
        value = str(fields.get(key, "")).strip()
        if value:
            lines.append(f"• {key}: {value}")
    return "\n".join(lines)


def is_allowed(chat_id: str) -> bool:
    if not ALLOWED_CHAT_IDS:
        return True
    return chat_id in ALLOWED_CHAT_IDS


def call_api(path: str):
    try:
        r = requests.get(f"{API_BASE_URL}{path}", timeout=6)
        return r.status_code, r.text
    except Exception as exc:
        return 0, str(exc)


def cmd_health(chat_id: str):
    st_live, body_live = call_api("/health/live")
    st_ready, body_ready = call_api("/health/ready")
    msg = (
        "Health\n"
        f"- live: {st_live}\n"
        f"- ready: {st_ready}\n"
        f"- ready_body: {body_ready[:500]}"
    )
    send_message(chat_id, msg)


def container_status(name: str):
    try:
        c = docker_client.containers.get(name)
        return f"{name}: {c.status}"
    except Exception as exc:
        return f"{name}: error ({exc})"


def cmd_status(chat_id: str):
    names = [
        "iiot_go_api",
        "iiot_emqx",
        "iiot_postgres",
        "iiot_timescaledb",
        "iiot_redis",
        "iiot_prometheus",
        "iiot_alertmanager",
    ]
    lines = ["Status"]
    for n in names:
        lines.append(f"- {container_status(n)}")
    send_message(chat_id, "\n".join(lines))


def tail_logs(container_name: str, lines: int = 40):
    c = docker_client.containers.get(container_name)
    output = c.logs(tail=lines).decode("utf-8", errors="replace")
    if len(output) > 3500:
        output = output[-3500:]
    return output or "(sem logs)"


def cmd_logs(chat_id: str, target: str):
    mapping = {
        "api": "iiot_go_api",
        "emqx": "iiot_emqx",
        "postgres": "iiot_postgres",
        "timescale": "iiot_timescaledb",
        "redis": "iiot_redis",
    }
    cname = mapping.get(target)
    if not cname:
        send_message(chat_id, "Uso: /logs api|emqx|postgres|timescale|redis")
        return
    try:
        out = tail_logs(cname, 40)
        send_message(chat_id, f"Logs {target} (tail 40)\n{out}")
    except Exception as exc:
        send_message(chat_id, f"Erro ao ler logs de {target}: {exc}")


def parse_metric_value(metrics_text: str, metric_name: str):
    m = re.search(rf"^{re.escape(metric_name)}\s+([0-9eE+\-.]+)$", metrics_text, re.MULTILINE)
    return m.group(1) if m else "n/a"

def sum_metric_vector(metrics_text: str, metric_prefix: str):
    total = 0.0
    found = False
    for line in metrics_text.splitlines():
        if not line.startswith(metric_prefix + "{"):
            continue
        try:
            total += float(line.rsplit(" ", 1)[1])
            found = True
        except Exception:
            continue
    if not found:
        return "n/a"
    if total.is_integer():
        return str(int(total))
    return f"{total:.2f}"


def cmd_metrics(chat_id: str):
    st, body = call_api("/metrics")
    if st != 200:
        send_message(chat_id, f"Falha ao ler /metrics: status={st} body={body[:300]}")
        return

    ing = sum_metric_vector(body, "telemetry_ingested_total")
    rej = sum_metric_vector(body, "telemetry_rejected_total")

    # 5xx rate and p95 via Prometheus API would be better; here summary from counters only.
    send_message(
        chat_id,
        "Metrics summary\n"
        f"- telemetry_ingested_total: {ing}\n"
        f"- telemetry_rejected_total: {rej}\n"
        "- para latência p95/erro use Grafana/Prometheus",
    )


def get_db_conn():
    return psycopg2.connect(
        host=POSTGRES_HOST,
        port=POSTGRES_PORT,
        dbname=POSTGRES_DB,
        user=POSTGRES_USER,
        password=POSTGRES_PASSWORD,
        connect_timeout=5,
    )


def get_counters(cur):
    cur.execute("SELECT count(*), COALESCE(max(created_at), '1970-01-01'::timestamptz) FROM users;")
    users_count, users_max = cur.fetchone()
    cur.execute("SELECT count(*), COALESCE(max(created_at), '1970-01-01'::timestamptz) FROM devices;")
    dev_count, dev_max = cur.fetchone()
    return {
        "users_count": int(users_count),
        "users_max": users_max,
        "devices_count": int(dev_count),
        "devices_max": dev_max,
    }


def get_new_users(cur, since_ts):
    cur.execute(
        """
        SELECT user_id, tenant_id, email, role, created_at
        FROM users
        WHERE created_at > %s
        ORDER BY created_at ASC
        LIMIT 20
        """,
        (since_ts,),
    )
    return cur.fetchall()


def get_new_devices(cur, since_ts):
    cur.execute(
        """
        SELECT device_id, tenant_id, device_label, status, created_at
        FROM devices
        WHERE created_at > %s
        ORDER BY created_at ASC
        LIMIT 20
        """,
        (since_ts,),
    )
    return cur.fetchall()


def fmt_ts(ts):
    try:
        return ts.astimezone(ZoneInfo(TELEGRAM_TIMEZONE)).strftime("%d/%m/%Y %H:%M:%S")
    except Exception:
        return str(ts)


def write_notification_audit(tenant_id, user_id, event_type, metadata):
    try:
        with get_db_conn() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO audit_log (
                        tenant_id, user_id, event_type, event_category, severity,
                        actor_type, action, result, resource_type, metadata, timestamp
                    )
                    VALUES (
                        %s, %s, %s, 'operations', 'info',
                        'system', 'telegram_notify', 'success', 'telegram', %s::jsonb, NOW()
                    )
                    """,
                    (tenant_id, user_id, event_type, json.dumps(metadata)),
                )
                conn.commit()
    except Exception:
        # Never break bot flow for audit persistence errors.
        return


def watch_new_entities(default_chat_id: str, state: dict):
    try:
        with get_db_conn() as conn:
            with conn.cursor() as cur:
                cur_state = get_counters(cur)
    except Exception:
        return

    if not state:
        state.update(cur_state)
        return

    # Avoid duplicate notifications with backend-originated alerts.
    # Can be enabled explicitly via TELEGRAM_WATCH_NOTIFY_EVENTS=true.
    if not TELEGRAM_WATCH_NOTIFY_EVENTS:
        state.update(cur_state)
        return

    if cur_state["users_count"] > state["users_count"]:
        diff = cur_state["users_count"] - state["users_count"]
        with get_db_conn() as conn:
            with conn.cursor() as cur:
                rows = get_new_users(cur, state["users_max"])

        lines = [f"🧾 [USUARIO] Cadastro detectado (+{diff}, total={cur_state['users_count']})"]
        for row in rows[:5]:
            user_id, tenant_id, email, role, created_at = row
            lines.append(
                render_event(
                    "└ detalhe",
                    {
                        "email": email,
                        "role": role,
                        "user_id": str(user_id),
                        "tenant_id": str(tenant_id),
                        "horario": fmt_ts(created_at),
                    },
                )
            )
            write_notification_audit(
                tenant_id,
                user_id,
                "ops.user_created_notified",
                {
                    "chat_id": default_chat_id,
                    "email": email,
                    "role": role,
                    "user_id": str(user_id),
                    "tenant_id": str(tenant_id),
                    "created_at": fmt_ts(created_at),
                },
            )
        if len(rows) > 5:
            lines.append(f"- ... +{len(rows)-5} usuários")
        send_message(default_chat_id, "\n".join(lines))

    if cur_state["devices_count"] > state["devices_count"]:
        diff = cur_state["devices_count"] - state["devices_count"]
        with get_db_conn() as conn:
            with conn.cursor() as cur:
                rows = get_new_devices(cur, state["devices_max"])

        lines = [f"📟 [DEVICE] Cadastro detectado (+{diff}, total={cur_state['devices_count']})"]
        for row in rows[:5]:
            device_id, tenant_id, device_label, status, created_at = row
            lines.append(
                render_event(
                    "└ detalhe",
                    {
                        "tenant_id": str(tenant_id),
                        "device_id": str(device_id),
                        "device_label": str(device_label),
                        "status": str(status),
                        "source": "watcher_db",
                        "horario": fmt_ts(created_at),
                    },
                )
            )
            write_notification_audit(
                tenant_id,
                None,
                "ops.device_created_notified",
                {
                    "chat_id": default_chat_id,
                    "tenant_id": str(tenant_id),
                    "device_id": str(device_id),
                    "device_label": device_label,
                    "status": status,
                    "source": "watcher_db",
                    "created_at": fmt_ts(created_at),
                },
            )
        if len(rows) > 5:
            lines.append(f"- ... +{len(rows)-5} dispositivos")
        send_message(default_chat_id, "\n".join(lines))

    state.update(cur_state)


def handle_command(chat_id: str, text: str):
    if text.startswith("/health"):
        cmd_health(chat_id)
        return
    if text.startswith("/status"):
        cmd_status(chat_id)
        return
    if text.startswith("/metrics"):
        cmd_metrics(chat_id)
        return
    if text.startswith("/logs"):
        parts = text.split()
        target = parts[1].lower() if len(parts) > 1 else ""
        cmd_logs(chat_id, target)
        return
    if text.startswith("/help"):
        send_message(
            chat_id,
            "Comandos:\n"
            "/health\n"
            "/status\n"
            "/metrics\n"
            "/logs api|emqx|postgres|timescale|redis",
        )
        return


def main():
    offset = None
    last_watch = 0
    state = {}
    default_chat_id = next(iter(ALLOWED_CHAT_IDS), None)

    while True:
        try:
            params = {"timeout": 20}
            if offset is not None:
                params["offset"] = offset
            r = requests.get(f"{BASE}/getUpdates", params=params, timeout=25)
            data = r.json()
            if data.get("ok"):
                for upd in data.get("result", []):
                    offset = upd["update_id"] + 1
                    msg = upd.get("message")
                    if not msg:
                        continue
                    chat_id = str(msg["chat"]["id"])
                    text = msg.get("text", "").strip()
                    if not text:
                        continue
                    if not is_allowed(chat_id):
                        continue
                    if default_chat_id is None:
                        default_chat_id = chat_id
                    handle_command(chat_id, text)

            now = int(time.time())
            if default_chat_id and now - last_watch >= WATCH_SECONDS:
                watch_new_entities(default_chat_id, state)
                last_watch = now

        except Exception:
            time.sleep(POLL_SECONDS)
            continue

        time.sleep(POLL_SECONDS)


if __name__ == "__main__":
    main()
