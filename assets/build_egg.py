import errno
import glob
import os
import tempfile
import sys
from shutil import copyfile
from subprocess import check_call, CalledProcessError
from configparser import ConfigParser

_SETUP_PY_TEMPLATE = """# Automatically created by: goscrapyd x scrapyd-client

from setuptools import setup, find_packages

setup(
    name         = 'project',
    version      = '1.0',
    packages     = find_packages(),
    entry_points = {'scrapy': ['settings = %(settings)s']},
)
"""

def get_config(sources):
    """Get Scrapy config file as a ConfigParser"""
    cfg = ConfigParser()
    cfg.read(sources)

    # Debug: print out sections to ensure we're reading the file correctly
    print(f"Config Sections: {cfg.sections()}")

    # Check if the 'settings' section exists
    if not cfg.has_section('settings'):
        raise ValueError(f"No section: 'settings' found in {sources}")

    return cfg

def retry_on_eintr(func, *args, **kw):
    """Run a function and retry it while getting EINTR errors"""
    while True:
        try:
            return func(*args, **kw)
        except IOError as e:
            if e.errno != errno.EINTR:
                raise

def _build_egg(scrapy_cfg_path):
    cwd = os.getcwd()
    try:
        os.chdir(os.path.dirname(scrapy_cfg_path))

        if os.path.exists('setup.py'):
            copyfile('setup.py', 'setup_backup.py')

        # Ensure the 'settings' section is found, otherwise raise an error
        settings = get_config(scrapy_cfg_path).get('settings', 'default')
        _create_default_setup_py(settings=settings)

        # Create a temporary directory to build the egg file
        d = tempfile.mkdtemp(prefix="goscrapyd-deploy-")
        retry_on_eintr(check_call, [sys.executable, 'setup.py', 'clean', '-a', 'bdist_egg', '-d', d])

        egg = glob.glob(os.path.join(d, '*.egg'))[0]
        return egg, d
    except Exception as err:
        print(f"Error: {err}", file=sys.stderr)
    finally:
        os.chdir(cwd)

def _create_default_setup_py(**kwargs):
    """Creates a default setup.py file for the project."""
    with open('setup.py', 'w') as f:
        content = _SETUP_PY_TEMPLATE % kwargs
        f.write(content)

def build_egg(scrapy_cfg_path):
    """Build the egg and print its content."""
    try:
        egg, tmpdir = _build_egg(scrapy_cfg_path)
    except Exception as err:
        # This will never be reached because we call sys.exit() above in case of error
        print(f"Error: {err}", file=sys.stderr)
        sys.exit(1)

    # Output the egg contents
    with open(egg, 'rb') as f:
        sys.stdout.buffer.write(f.read())

    # Clean up the temporary files
    os.remove(egg)
    os.rmdir(tmpdir)

# Standalone execution
if __name__ == "__main__":
    # Check if the script is being executed with the proper arguments
    if len(sys.argv) != 2:
        raise Exception("Usage: python build_egg.py <path_to_scrapy_cfg>")

    scrapy_cfg_path = sys.argv[1]
    build_egg(scrapy_cfg_path)
