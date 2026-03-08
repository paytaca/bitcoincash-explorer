from fabric import task, Connection
from patchwork.transfers import rsync  # type: ignore
import os

# Global configuration dictionary
CONFIG = {
    "HOST": None,
    "USER": None,
    "PATH": "/root/bitcoincash-explorer",
    "COMPOSE_FILE": "docker-compose.yml",
    "COMPOSE_PROJECT": "bitcoincash_explorer",
    "ENV_FILE": ".env",  # Default to .env
}


def _load_config(env_path: str) -> None:
    """
    Loads configuration from a specific .env file into CONFIG.
    """
    if not os.path.exists(env_path):
        print(f"⚠️ Warning: Configuration file '{env_path}' not found.")
        return

    # Update ENV_FILE in config so sync knows which one to upload
    CONFIG["ENV_FILE"] = env_path

    with open(env_path, "r", encoding="utf-8") as f:
        for raw in f:
            line = raw.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            k = k.strip()
            v = v.strip().strip("'").strip('"')

            # Map environment variables to CONFIG keys
            if k == "SERVER_HOSTNAME":
                CONFIG["HOST"] = v
            elif k == "SERVER_USER":
                CONFIG["USER"] = v
            elif k == "SERVER_PATH":
                CONFIG["PATH"] = v
            elif k == "SERVER_DOCKER_COMPOSE_FILE":
                CONFIG["COMPOSE_FILE"] = v
            elif k == "SERVER_DOCKER_COMPOSE_PROJECT":
                CONFIG["COMPOSE_PROJECT"] = v


def _require_server_config() -> None:
    missing = []
    if not CONFIG["HOST"]:
        missing.append("SERVER_HOSTNAME (in .env.mainnet or .env.chipnet)")
    if not CONFIG["USER"]:
        missing.append("SERVER_USER (in .env.mainnet or .env.chipnet)")
    if missing:
        raise RuntimeError(f"Missing required configuration: {', '.join(missing)}")


def get_connection() -> Connection:
    """Helper function to create connection"""
    _require_server_config()
    return Connection(host=CONFIG["HOST"], user=CONFIG["USER"])


@task
def mainnet(c):
    """Configure deployment for Mainnet (uses .env.mainnet)"""
    _load_config(".env.mainnet")
    print(f"🌍 Selected environment: Mainnet ({CONFIG['HOST']})")


@task
def chipnet(c):
    """Configure deployment for Chipnet (uses .env.chipnet)"""
    _load_config(".env.chipnet")
    print(f"🧪 Selected environment: Chipnet ({CONFIG['HOST']})")


@task
def uname(c):
    """Test connection by running uname"""
    conn = get_connection()
    conn.run("uname -a")


@task
def sync(c):
    """Sync local files to remote server using rsync"""
    conn = c.config.run.env.get("conn") or get_connection()

    # Sync everything excluding local env files (we handle it separately)
    rsync(
        conn,
        ".",
        CONFIG["PATH"],
        exclude=[
            ".git/",
            ".venv/",
            ".DS_Store",
            ".env",  # Exclude local .env
            ".env.local",  # Exclude local env
            ".env.mainnet",  # Exclude mainnet env (uploaded separately)
            ".env.chipnet",  # Exclude chipnet env
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

    # Upload the selected environment file as .env on the remote server
    source_env = CONFIG["ENV_FILE"]
    if os.path.exists(source_env):
        print(f"📄 Uploading {source_env} to {CONFIG['PATH']}/.env")
        conn.put(source_env, f"{CONFIG['PATH']}/.env")
    else:
        print(f"⚠️ Warning: Environment file {source_env} not found locally!")


@task
def build(c, no_cache=False):
    """Build Docker image (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    cache_flag = " --no-cache" if str(no_cache).lower() in ["1", "true", "yes"] else ""
    with conn.cd(CONFIG["PATH"]):
        conn.run(
            f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} build{cache_flag}"
        )


@task
def up(c):
    """Start Docker containers (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(CONFIG["PATH"]):
        conn.run(
            f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} up -d"
        )


@task
def down(c):
    """Stop Docker containers (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(CONFIG["PATH"]):
        conn.run(
            f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} down"
        )


@task
def restart(c):
    """Restart Docker containers (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(CONFIG["PATH"]):
        conn.run(
            f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} restart"
        )


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
    with conn.cd(CONFIG["PATH"]):
        if follow:
            conn.run(
                f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} logs -f --tail=100"
            )
        else:
            conn.run(
                f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} logs --tail=100"
            )


@task
def status(c):
    """Check Docker container status (remote)"""
    conn = c.config.run.env.get("conn") or get_connection()
    with conn.cd(CONFIG["PATH"]):
        conn.run(
            f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} ps"
        )


@task
def clear_redis(c):
    """Clear Redis lists (blocks and transactions)"""
    conn = c.config.run.env.get("conn") or get_connection()
    print("🧹 Clearing Redis lists...")
    with conn.cd(CONFIG["PATH"]):
        # Delete the Redis lists for blocks and transactions
        conn.run(
            f"sudo docker-compose -p {CONFIG['COMPOSE_PROJECT']} -f {CONFIG['COMPOSE_FILE']} exec -T redis redis-cli DEL bch:blocks:latest bch:txs:latest bch:mempool:txids || true"
        )
    print("✅ Redis lists cleared")


@task
def clear_cache(c):
    """Clear server-side memory cache (Go version has no persistent cache to clear)"""
    print("🧹 Server cache cleared (Go API has no persistent in-memory cache)")


@task
def deploy(c):
    """Full deployment: sync, build, down, up"""
    # Default to mainnet if no environment selected
    if not CONFIG["HOST"] and os.path.exists(".env.mainnet"):
        print("ℹ️  No environment selected. Defaulting to Mainnet (.env.mainnet).")
        _load_config(".env.mainnet")

    print("🚀 Starting deployment...")
    sync(c)
    print("📦 Building Docker image...")
    build(c, no_cache=False)
    print("🛑 Stopping old containers...")
    down(c)
    print("🎬 Starting new containers...")
    up(c)
    print("🧹 Clearing Redis lists...")
    clear_redis(c)
    print("🧹 Clearing server cache...")
    clear_cache(c)
    # print("📜 Streaming logs (Ctrl+C to exit)...")
    # logs(c, follow=True)
