#!/bin/bash
# Writes frontend/config/.env. Run from frontend/ (make init).

session_secret=$(openssl rand -hex 32)
# localhost, this machine's name and LAN addresses; edit WEB_HOSTS when the IP changes.
lan_ips=$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E '^(192\.168|10)\.')
# The machine's name too, plain and as mDNS (.local), for devices that type it.
web_hosts="localhost, $(hostname), $(hostname).local${lan_ips:+$(printf ', %s' ${lan_ips})}"
web_sni=$(echo "${lan_ips:-localhost}" | head -n 1)

echo "TZ='America/Manaus'                             # Set system time zone
WEB_VERSION='0.1.0'                             # Image tag
WEB_PORT='3016'                                 # Plain HTTP on this machine only (127.0.0.1)
WEB_HTTPS_PORT='3443'                           # HTTPS for every device: https://<this machine's IP>:3443
WEB_HOSTS='${web_hosts}'                        # Names and IPs people type, comma separated; the certificate covers them
WEB_SNI='${web_sni}'                            # Certificate for a browser that opens the bare IP (it sends no name): this machine's LAN IP
WEB_IP='172.30.98.10'                           # Container address; must equal WEB_IP of the backend stack

API_PORT='9979'                                 # Backend PORT; dev calls http://localhost:API_PORT, Docker http://backend:API_PORT
SESSION_SECRET='${session_secret}'              # Signs the session cookie - PRIVATE TOKEN
DEV_ALLOWED_ORIGINS=''                          # Dev server only: extra host names allowed to open it (comma separated); this machine's IPs are always allowed
COOKIE_SECURE=''                                # Secure cookie: empty = only when served over HTTPS, 1 forces it, 0 disables it
CLIENT_IP_HEADER=''                             # Header with the client IP set by a reverse proxy (default x-forwarded-for)
NEXT_PUBLIC_PLUGGY_CONNECTORS=''               # Pluggy connector ids the widget lists (comma separated); empty = all. Trial account: 200,2 (MeuPluggy, sandbox)" >config/.env
