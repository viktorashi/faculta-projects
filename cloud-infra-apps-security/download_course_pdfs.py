#!/usr/bin/env python3
#USAGE include file="download-guides.usage.kdl"
"""
AWS Academy Course PDF Downloader
Automates downloading course Student Guides from AWS Academy (Canvas LMS + ContentController LTI)
using an existing Opera GX browser session.
"""

import argparse
import glob
import hashlib
import http.cookiejar
import json
import os
import re
import shutil
import sqlite3
import subprocess
import sys
import time
import urllib.parse
import urllib.request
import zlib

DEFAULT_COURSE_ID = "192250"
DEFAULT_OUTPUT_DIR = os.path.expanduser("~/Downloads/AWS Academy Cloud Security Foundations")
DEFAULT_BROWSER = "Opera GX"
DEFAULT_CACHE_DIR = os.path.expanduser("~/Library/Caches/com.operasoftware.OperaGX/Default/Cache/Cache_Data")
DEFAULT_COOKIES_PATH = os.path.expanduser("~/Library/Application Support/com.operasoftware.OperaGX/Default/Cookies")
FALLBACK_PASSWORD = "qsoDpUP5LE9gAXU18cFfxA=="

# Fallback module definitions if Canvas API is unavailable
FALLBACK_MODULES = [
    {"num": 1, "name": "Module 1: Welcome to AWS Academy Cloud Security Foundations", "item_id": "19023128"},
    {"num": 2, "name": "Module 2: Introduction to Security on AWS", "item_id": "19023138"},
    {"num": 3, "name": "Module 3: Securing Access to Cloud Resources", "item_id": "19023152"},
    {"num": 4, "name": "Module 4: Securing Your Infrastructure", "item_id": "19023181"},
    {"num": 5, "name": "Module 5: Protecting Data in Your Application", "item_id": "19023220"},
    {"num": 6, "name": "Module 6: Logging and Monitoring", "item_id": "19023254"},
    {"num": 7, "name": "Module 7: Responding to and Managing an Incident", "item_id": "19023284"},
    {"num": 8, "name": "Module 8: Bridging to Certification", "item_id": "19023307"},
]


def find_openssl_binary():
    """Locate openssl executable."""
    candidate = shutil.which("openssl")
    if candidate:
        return candidate
    for path in ["/opt/homebrew/bin/openssl", "/usr/bin/openssl", "/usr/local/bin/openssl"]:
        if os.path.isfile(path) and os.access(path, os.X_OK):
            return path
    return "openssl"


def get_opera_storage_password():
    """Retrieve Opera Safe Storage password from macOS keychain."""
    try:
        res = subprocess.run(
            ["security", "find-generic-password", "-wa", "Opera", "-s", "Opera Safe Storage"],
            capture_output=True,
            text=True,
            check=True,
            timeout=5,
        )
        pw = res.stdout.strip()
        if pw:
            return pw
    except Exception:
        pass
    return FALLBACK_PASSWORD


def decrypt_opera_cookies(cookies_path=DEFAULT_COOKIES_PATH, domain_filter="instructure.com"):
    """Decrypt Opera GX SQLite cookies for the specified domain."""
    if not os.path.exists(cookies_path):
        raise FileNotFoundError(f"Opera GX Cookies database not found at {cookies_path}")

    pw = get_opera_storage_password().encode("utf-8")
    key = hashlib.pbkdf2_hmac("sha1", pw, b"saltysalt", 1003, dklen=16).hex()
    iv = (b" " * 16).hex()
    openssl_bin = find_openssl_binary()

    tmp_db = f"/tmp/opera_cookies_{os.getpid()}_{int(time.time())}.db"
    shutil.copyfile(cookies_path, tmp_db)

    cookies = {}
    try:
        conn = sqlite3.connect(tmp_db)
        cursor = conn.cursor()
        cursor.execute("SELECT name, encrypted_value FROM cookies WHERE host_key LIKE ?", (f"%{domain_filter}%",))
        for name, enc_val in cursor.fetchall():
            if enc_val.startswith(b"v10"):
                raw_cipher = enc_val[3:]
                p = subprocess.Popen(
                    [openssl_bin, "enc", "-d", "-aes-128-cbc", "-K", key, "-iv", iv],
                    stdin=subprocess.PIPE,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                )
                out, _ = p.communicate(raw_cipher)
                cookies[name] = out[32:].decode("utf-8", "ignore")
        conn.close()
    finally:
        if os.path.exists(tmp_db):
            os.remove(tmp_db)

    return cookies


def fetch_course_modules(course_id, canvas_cookies):
    """Fetch module and student guide items via Canvas API, falling back to static list if needed."""
    cookie_hdr = "; ".join(f"{k}={v}" for k, v in canvas_cookies.items())
    url = f"https://awsacademy.instructure.com/api/v1/courses/{course_id}/modules?include[]=items&per_page=100"
    req = urllib.request.Request(
        url,
        headers={
            "Cookie": cookie_hdr,
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
        },
    )

    modules_to_download = []
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = json.loads(resp.read().decode("utf-8"))

        mod_num = 1
        for m in data:
            m_name = m.get("name", "")
            for item in m.get("items", []):
                title = item.get("title", "")
                if "student guide" in title.lower():
                    # Extract number from module title if present
                    match_num = re.search(r"Module\s*(\d+)", m_name, re.IGNORECASE)
                    current_num = int(match_num.group(1)) if match_num else mod_num
                    modules_to_download.append(
                        {
                            "num": current_num,
                            "name": m_name,
                            "item_id": str(item.get("id")),
                            "pdf_keyword": f"M{current_num:02d}",
                        }
                    )
                    mod_num += 1
    except Exception as e:
        print(f"[!] Warning: Canvas API query failed ({e}). Using default course structure.")

    if not modules_to_download and course_id == DEFAULT_COURSE_ID:
        for m in FALLBACK_MODULES:
            modules_to_download.append(
                {
                    "num": m["num"],
                    "name": m["name"],
                    "item_id": m["item_id"],
                    "pdf_keyword": f"M{m['num']:02d}",
                }
            )

    return modules_to_download


def get_launch_url_for_item(course_id, item_id, canvas_cookies):
    """Perform LTI 1.3 launch handshake and return ContentController ltiDispatch URL."""
    cj = http.cookiejar.CookieJar()
    for k, v in canvas_cookies.items():
        ck = http.cookiejar.Cookie(
            version=0,
            name=k,
            value=v,
            port=None,
            port_specified=False,
            domain="awsacademy.instructure.com",
            domain_specified=True,
            domain_initial_dot=False,
            path="/",
            path_specified=True,
            secure=True,
            expires=None,
            discard=True,
            comment=None,
            comment_url=None,
            rest={"HttpOnly": None},
            rfc2109=False,
        )
        cj.set_cookie(ck)

    class NoRedirectHandler(urllib.request.HTTPRedirectHandler):
        def http_error_301(self, req, fp, code, msg, headers):
            return fp

        def http_error_302(self, req, fp, code, msg, headers):
            return fp

        def http_error_303(self, req, fp, code, msg, headers):
            return fp

        def http_error_307(self, req, fp, code, msg, headers):
            return fp

    opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cj), NoRedirectHandler)
    cookie_header = "; ".join(f"{k}={v}" for k, v in canvas_cookies.items())
    base_item_url = f"https://awsacademy.instructure.com/courses/{course_id}/modules/items/{item_id}"

    # Step 1: GET Canvas item page to extract OIDC Initiation form
    req1 = urllib.request.Request(
        base_item_url,
        headers={
            "Cookie": cookie_header,
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
        },
    )
    with urllib.request.urlopen(req1, timeout=15) as resp1:
        html1 = resp1.read().decode("utf-8")

    m_form = re.search(r"<form[^>]*action=\"([^\"]+)\"[^>]*>(.*?)</form>", html1, re.DOTALL)
    if not m_form:
        return None

    action_url = m_form.group(1).replace("&amp;", "&")
    fields = {}
    for m_inp in re.finditer(r"<input[^>]*name=\"([^\"]+)\"[^>]*value=\"([^\"]*)\"", m_form.group(2)):
        fields[m_inp.group(1)] = m_inp.group(2).replace("&amp;", "&")

    # Step 2: POST to ContentController oidcLoginInitiation
    post_data = urllib.parse.urlencode(fields).encode("utf-8")
    req2 = urllib.request.Request(
        action_url,
        data=post_data,
        headers={
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
            "Referer": base_item_url,
            "Origin": "https://awsacademy.instructure.com",
        },
    )
    resp2 = opener.open(req2, timeout=15)
    loc2 = resp2.headers.get("Location")
    if not loc2:
        return None

    # Step 3: GET authorization redirect -> Canvas /api/lti/authorize
    req3 = urllib.request.Request(loc2, headers={"User-Agent": "Mozilla/5.0", "Referer": base_item_url})
    resp3 = opener.open(req3, timeout=15)
    loc3 = resp3.headers.get("Location")
    if not loc3:
        return None

    # Step 4: GET authorization form on awsacademy
    req4 = urllib.request.Request(loc3, headers={"User-Agent": "Mozilla/5.0", "Referer": loc2})
    resp4 = opener.open(req4, timeout=15)
    body4 = resp4.read().decode("utf-8")

    m_form4 = re.search(r"<form[^>]*action=\"([^\"]+)\"[^>]*>(.*?)</form>", body4, re.DOTALL)
    if not m_form4:
        return None

    action4 = m_form4.group(1).replace("&amp;", "&")
    fields4 = {}
    for m_inp in re.finditer(r"<input[^>]*name=\"([^\"]+)\"[^>]*value=\"([^\"]*)\"", m_form4.group(2)):
        fields4[m_inp.group(1)] = m_inp.group(2).replace("&amp;", "&")

    # Step 5: POST id_token to ContentController /launches
    post_data4 = urllib.parse.urlencode(fields4).encode("utf-8")
    req5 = urllib.request.Request(
        action4,
        data=post_data4,
        headers={
            "User-Agent": "Mozilla/5.0",
            "Referer": loc3,
            "Origin": "https://awsacademy.instructure.com",
        },
    )
    resp5 = opener.open(req5, timeout=15)
    loc5 = resp5.headers.get("Location")
    if not loc5:
        return None

    # Step 6: Follow redirect to get final ltiDispatch URL
    req6 = urllib.request.Request(loc5, headers={"User-Agent": "Mozilla/5.0", "Referer": "https://awsacademy.instructure.com"})
    resp6 = opener.open(req6, timeout=15)
    final_url = resp6.headers.get("Location")
    return final_url or loc5


def find_pdf_in_cache(pdf_keyword, cache_dir=DEFAULT_CACHE_DIR):
    """Scan browser cache safely and extract uncompressed PDF matching the keyword."""
    if not os.path.exists(cache_dir):
        return None, None

    cache_entries = []
    try:
        with os.scandir(cache_dir) as it:
            for entry in it:
                if entry.name.endswith("_0"):
                    try:
                        cache_entries.append((entry.stat().st_mtime, entry.path))
                    except OSError:
                        pass
    except OSError:
        pass

    cache_entries.sort(key=lambda x: x[0], reverse=True)

    for _, path in cache_entries[:300]:
        try:
            with open(path, "rb") as fp:
                data = fp.read()
            if b".pdf" not in data.lower():
                continue

            key_len = int.from_bytes(data[12:16], "little")
            key = data[24 : 24 + key_len].decode("utf-8", "ignore")
            filename = os.path.basename(urllib.parse.unquote(key.split(" ")[-1]))

            if filename.lower().endswith(".pdf") and pdf_keyword.lower() in filename.lower():
                payload_start = 24 + key_len
                http_idx = data.find(b"HTTP/1.1")
                payload = data[payload_start:http_idx]
                try:
                    pdf_data = zlib.decompress(payload, 16 + zlib.MAX_WBITS)
                except Exception:
                    pdf_data = payload

                if pdf_data.startswith(b"%PDF"):
                    return filename, pdf_data
        except Exception:
            pass

    return None, None


def download_course_pdfs(course_id=DEFAULT_COURSE_ID, output_dir=DEFAULT_OUTPUT_DIR, browser=DEFAULT_BROWSER, timeout=30, force=False):
    """Main workflow to retrieve and save all course Student Guides."""
    os.makedirs(output_dir, exist_ok=True)
    print(f"[*] Starting AWS Academy Course PDF download...")
    print(f"[*] Course ID : {course_id}")
    print(f"[*] Output Dir: {output_dir}")
    print(f"[*] Browser   : {browser}\n")

    print("[*] Decrypting Canvas LMS session cookies from Opera GX...")
    canvas_cookies = decrypt_opera_cookies()
    if not canvas_cookies:
        print("[!] Error: No Canvas cookies found. Ensure you are logged into AWS Academy in Opera GX.")
        sys.exit(1)
    print(f"    Found {len(canvas_cookies)} session cookies.")

    print("[*] Retrieving course modules...")
    modules = fetch_course_modules(course_id, canvas_cookies)
    print(f"    Found {len(modules)} module guide(s) to process.\n")

    results = []

    for mod in modules:
        num = mod["num"]
        name = mod["name"]
        item_id = mod["item_id"]
        kw = mod["pdf_keyword"]

        # Clean module title for directory name
        clean_title = re.sub(r"^Module\s*\d+\s*[:\-]\s*", "", name).strip()
        mod_dir = os.path.join(output_dir, f"Module {num:02d} - {clean_title}")
        os.makedirs(mod_dir, exist_ok=True)

        print(f"[{num}/{len(modules)}] Processing Module {num:02d}: {clean_title}")

        # Check existing file on disk
        existing = [f for f in os.listdir(mod_dir) if f.lower().endswith(".pdf") and os.path.getsize(os.path.join(mod_dir, f)) > 100000]
        if existing and not force:
            filepath = os.path.join(mod_dir, existing[0])
            size = os.path.getsize(filepath)
            print(f"    [OK] Already present: {existing[0]} ({size:,} bytes)")
            results.append((num, existing[0], filepath, size, "Cached on disk"))
            continue

        # Check if already in browser cache
        fname, pdf_data = find_pdf_in_cache(kw)
        if not pdf_data:
            print(f"    Authenticating LTI launch for item {item_id}...")
            launch_url = get_launch_url_for_item(course_id, item_id, canvas_cookies)
            if not launch_url:
                print(f"    [!] Failed to generate LTI launch URL for item {item_id}")
                results.append((num, "-", "-", 0, "Launch failed"))
                continue

            print(f"    Opening in {browser} to load PDF into cache...")
            subprocess.run(["open", "-a", browser, launch_url])

            # Poll cache
            start_t = time.time()
            while time.time() - start_t < timeout:
                time.sleep(2)
                fname, pdf_data = find_pdf_in_cache(kw)
                if pdf_data:
                    break

        if pdf_data and fname:
            target_path = os.path.join(mod_dir, fname)
            with open(target_path, "wb") as f_out:
                f_out.write(pdf_data)
            size = len(pdf_data)
            print(f"    [SUCCESS] Saved {fname} ({size:,} bytes)")
            results.append((num, fname, target_path, size, "Downloaded"))
        else:
            print(f"    [!] Timed out waiting for {kw} PDF in cache")
            results.append((num, "-", "-", 0, "Timeout"))

    print("\n" + "=" * 60)
    print("DOWNLOAD SUMMARY")
    print("=" * 60)
    for num, fname, path, size, status in results:
        status_icon = "✓" if size > 0 else "✗"
        print(f" {status_icon} Module {num:02d}: {fname:<30} ({size:>9,} bytes) [{status}]")
    print("=" * 60)
    print(f"Target Directory: {output_dir}\n")


def main():
    # When run via mise task with a usage spec, arguments are passed as usage_* env vars:
    env_course_id = os.environ.get("usage_course_id")
    env_output_dir = os.environ.get("usage_output_dir")
    env_browser = os.environ.get("usage_browser")
    env_timeout = os.environ.get("usage_timeout")
    env_force = os.environ.get("usage_force") == "true"

    parser = argparse.ArgumentParser(description="Download AWS Academy Student Guides via Opera GX session")
    parser.add_argument("-c", "--course-id", default=env_course_id or DEFAULT_COURSE_ID, help=f"Canvas Course ID (default: {DEFAULT_COURSE_ID})")
    parser.add_argument("-o", "--output-dir", default=env_output_dir or DEFAULT_OUTPUT_DIR, help=f"Destination directory (default: {DEFAULT_OUTPUT_DIR})")
    parser.add_argument("-b", "--browser", default=env_browser or DEFAULT_BROWSER, help=f"Browser application (default: {DEFAULT_BROWSER})")
    parser.add_argument("-t", "--timeout", type=int, default=int(env_timeout) if env_timeout else 30, help="Wait timeout in seconds per module (default: 30)")
    parser.add_argument("-f", "--force", action="store_true", default=env_force, help="Force re-download even if already present")
    args = parser.parse_args()

    download_course_pdfs(
        course_id=args.course_id,
        output_dir=os.path.expanduser(args.output_dir),
        browser=args.browser,
        timeout=args.timeout,
        force=args.force,
    )


if __name__ == "__main__":
    main()
