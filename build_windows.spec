# -*- mode: python ; coding: utf-8 -*-
import sys
from PyInstaller.utils.hooks import collect_data_files, collect_submodules

block_cipher = None

# Collect hidden imports for dynamic libraries
hidden_imports = [
    'sounddevice',
    'soundfile',
    'numpy',
    'pydub',
    'adhanpy',
    'requests',
    'pystray',
    'PIL',
    'plyer',
    'plyer.platforms.win.notification',
    'apscheduler',
    'apscheduler.triggers.cron',
    'apscheduler.triggers.interval',
    'sqlite3',
    'pytz',
    'flask',
    'jinja2',
    'werkzeug'
]

datas = [
    ('templates', 'templates'),
]

# If static folder exists, include it
import os
if os.path.exists('static'):
    datas.append(('static', 'static'))

a = Analysis(
    ['main.py'],
    pathex=[],
    binaries=[],
    datas=datas,
    hiddenimports=hidden_imports,
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    win_no_prefer_redirects=False,
    win_private_assemblies=False,
    cipher=block_cipher,
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.zipfiles,
    a.datas,
    [],
    name='SmartOfficeAnnouncer',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=False,  # Set to False for headless background execution
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
    icon=None
)
