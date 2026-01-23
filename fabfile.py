from fabric import task, Connection
from patchwork.transfers import rsync  # type: ignore
import os


def _load_dotenv(path: str = ".env") -> None:
    """
    Minimal .env loader (no external dependency).
    - Does not override existing environment variables.
    - Supports KEY=VALUE, ignores comments/blank lines.
    """
    try:
        with open(path, "r", encoding="utf-8") as f:
            for raw in f:
                line = raw.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, v = line.split("=", 1)
                k = k.strip()
                v = v.strip().strip("'").strip('"')
                if k and k not in os.environ:
                    os.environ[k] = v
    except FileNotFoundError:
        # OK: allow env-only deployments
        return


_load_dotenv()

# Configuration (server is sourced from .env)
REMOTE_HOST = os.environ.get("SERVER_HOSTNAME", "").strip()
REMOTE_USER = os.environ.get("SERVER_USER", "").strip()

# Optional: override paths if you want
REMOTE_PATH = os.environ.get("SERVER_PATH", "/root/bitcoincash-explorer").strip()
COMPOSE_FILE = os.environ.get("SERVER_DOCKER_COMPOSE_FILE", "docker-compose.prod.yml").strip()
COMPOSE_PROJECT = os.environ.get("SERVER_DOCKER_COMPOSE_PROJECT", "bitcoincash_explorer").strip()


def _require_server_config() -> None:
    missing = []
    if not REMOTE_HOST:
        missing.append("SERVER_HOSTNAME")
    if not REMOTE_USER:
        missing.append("SERVER_USER")
    if missing:
        raise RuntimeError(f"Missing required .env/env vars: {', '.join(missing)}")


def get_connection() -> Connection:
    """Helper function to create connection"""
    _require_server_config()
    return Connection(host=REMOTE_HOST, user=REMOTE_USER)


@task
def prod(c):
    """Configure connection for subsequent tasks (fab prod <task>)"""
    _require_server_config()
    conn = c.config.run.env["conn"] = Connection(REMOTE_HOST, user=REMOTE_USER)
    return conn


@task
def uname(c):
    """Test connection by running uname"""
    conn = get_connection()
    conn.run("uname -a")


@task
def sync(c):
    """Sync local files to remote server using rsync"""
    conn = c.config.run.env.get("conn") or get_connection()
    rsync(
        conn,
        ".",
        REMOTE_PATH,
        exclude=[
            # patchwork.transfers.rsync expects exclude *patterns* (it adds --exclude=...)
            ".git/",
            ".venv/",
            ".DS_Store",
            # NOTE: we intentionally sync `.env` so docker-compose `env_file: .env` works in production.
            # `.env` is gitignored, but it must exist on the server.
            "__pycache__/",
            "node_modules/",
            "dist/",
            ".nuxt/",
            ".output/",
            ".nitro/",
            ".npm-cache/",
            ".cache/",
            ".vscode/",
            ".idea/",
        ],
    )


@task
def build(c):
    """Build Docker image (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(REMOTE_PATH):
        conn.run(f"sudo docker-compose -p {COMPOSE_PROJECT} -f {COMPOSE_FILE} build")


@task
def up(c):
    """Start Docker containers (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(REMOTE_PATH):
        conn.run(f"sudo docker-compose -p {COMPOSE_PROJECT} -f {COMPOSE_FILE} up -d --build --force-recreate")


@task
def down(c):
    """Stop Docker containers (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(REMOTE_PATH):
        conn.run(f"sudo docker-compose -p {COMPOSE_PROJECT} -f {COMPOSE_FILE} down")


@task
def restart(c):
    """Restart Docker containers (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(REMOTE_PATH):
        conn.run(f"sudo docker-compose -p {COMPOSE_PROJECT} -f {COMPOSE_FILE} restart")


@task
def prune(c):
    """Prune unused Docker images and networks (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    conn.run("sudo docker image prune -f")
    conn.run("sudo docker network prune -f")
    print("✅ Docker cleanup complete")


@task
def logs(c, follow=True):
    """View Docker logs (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(REMOTE_PATH):
        if follow:
            conn.run(f"sudo docker-compose -p {COMPOSE_PROJECT} -f {COMPOSE_FILE} logs -f --tail=100")
        else:
            conn.run(f"sudo docker-compose -p {COMPOSE_PROJECT} -f {COMPOSE_FILE} logs --tail=100")


@task
def status(c):
    """Check Docker container status (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(REMOTE_PATH):
        conn.run(f"sudo docker-compose -p {COMPOSE_PROJECT} -f {COMPOSE_FILE} ps")


@task
def deploy(c):
    """Full deployment: sync, build, down, up"""
    print("🚀 Starting deployment...")
    sync(c)
    print("📦 Building Docker image...")
    build(c)
    print("🛑 Stopping old containers...")
    down(c)
    print("🎬 Starting new containers...")
    up(c)

