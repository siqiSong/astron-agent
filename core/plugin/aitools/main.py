"""
AI Tools service main entry module
"""

import functools
import os
import subprocess
import sys
import time
from pathlib import Path

print = functools.partial(print, flush=True)  # pylint: disable=redefined-builtin
os.environ["PYTHONWARNINGS"] = "ignore:pkg_resources is deprecated"


def setup_python_path() -> None:
    """Set up Python path to include root, parent dir, and grandparent dir"""
    # Retrieve the path of the current script and the root directory.
    current_file_path = Path(__file__)
    project_root = current_file_path.parent  # Project root directory
    parent_dir = project_root.parent  # Parent directory
    grandparent_dir = parent_dir.parent

    # Retrieve the current PYTHONPATH
    python_path = os.environ.get("PYTHONPATH", "")

    # Check and add the necessary directories.
    new_paths = []
    for directory in [project_root, parent_dir, grandparent_dir]:
        if Path(directory).exists() and str(directory) not in python_path:
            new_paths.append(str(directory))

    # If there is a path that needs to be added, update the PYTHONPATH.
    if new_paths:
        new_paths_str = os.pathsep.join(new_paths)
        if python_path:
            os.environ["PYTHONPATH"] = f"{new_paths_str}{os.pathsep}{python_path}"
        else:
            os.environ["PYTHONPATH"] = new_paths_str
        print(f"🔧 PYTHONPATH: {os.environ['PYTHONPATH']}")


def start_service() -> None:
    """Start and supervise the FastAPI and tabletools services."""
    print("\n🚀 Starting AITools service...")

    try:
        relative_path = (Path(__file__).resolve().parent).relative_to(
            Path.cwd()
        ) / "app/start_server.py"
        if not relative_path.exists():
            raise FileNotFoundError(f"can not find {relative_path}")
        tabletools_path = Path(os.environ.get("TABLETOOLS_BINARY", "/usr/local/bin/tabletools"))
        if not tabletools_path.exists():
            raise FileNotFoundError(f"can not find {tabletools_path}")

        processes = [
            subprocess.Popen([sys.executable, relative_path]),
            subprocess.Popen([str(tabletools_path)]),
        ]
        while True:
            for process in processes:
                exit_code = process.poll()
                if exit_code is None:
                    continue
                for sibling in processes:
                    if sibling is not process and sibling.poll() is None:
                        sibling.terminate()
                        sibling.wait(timeout=10)
                if exit_code:
                    print(f"❌ Service child exited with code {exit_code}")
                    sys.exit(exit_code)
                return
            time.sleep(0.2)
    except KeyboardInterrupt:
        print("\n🛑 Service stopped")
        for process in locals().get("processes", []):
            if process.poll() is None:
                process.terminate()
        sys.exit(0)


def main() -> None:
    """Main function"""
    print("🌟 AITools Development Environment Launcher")
    print("=" * 50)

    # Set up Python path
    setup_python_path()

    # Load environment configuration
    config_file = Path(__file__).parent / "config.env"
    os.environ["CONFIG_FILE"] = str(config_file)

    # Start service
    start_service()


if __name__ == "__main__":
    main()
