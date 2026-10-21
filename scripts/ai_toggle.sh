#!/bin/bash
# ==============================================================================
# AI Control CLI Tool for Psikologi Kanagata Apps
# Allows enabling or disabling AI generation on-demand via terminal command.
# When disabled, generation requests will output: 'Token AI sudah habis'.
# ==============================================================================

set -e

ACTION="${1:-status}"
ACTION=$(echo "$ACTION" | tr '[:upper:]' '[:lower:]')

# Flag file locations
CONTAINER_FLAG="/app/data/ai_cache/AI_DISABLED"
HOST_FLAG_VOLUME="/var/lib/docker/volumes/psikologi-kanagata-apps_aicache/_data/AI_DISABLED"
LOCAL_FLAG="data/ai_cache/AI_DISABLED"

# Detect if running inside docker container
IS_CONTAINER=0
if [ -f /.dockerenv ] || [ -d /app/data ]; then
    IS_CONTAINER=1
fi

case "$ACTION" in
    off|disable|matikan|stop)
        if [ "$IS_CONTAINER" -eq 1 ]; then
            mkdir -p /app/data/ai_cache 2>/dev/null || true
            touch "$CONTAINER_FLAG"
        else
            # On host
            if [ -d "/var/lib/docker/volumes/psikologi-kanagata-apps_aicache/_data" ]; then
                touch "$HOST_FLAG_VOLUME"
            fi
            if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "psikologi_app"; then
                docker exec psikologi_app sh -c "mkdir -p /app/data/ai_cache && touch $CONTAINER_FLAG" 2>/dev/null || true
            fi
            mkdir -p data/ai_cache 2>/dev/null || true
            touch "$LOCAL_FLAG" 2>/dev/null || true
        fi

        echo ""
        echo "=========================================================="
        echo "  [AI GENERATION] STATUS: NONAKTIF (OFF)"
        echo "=========================================================="
        echo "  AI telah dinonaktifkan / dimatikan."
        echo "  Output sistem saat generate: 'Token AI sudah habis'."
        echo "  Tidak ada request yang dikirimkan ke Gemini API."
        echo "=========================================================="
        echo ""
        ;;

    on|enable|nyalakan|hidupkan|start)
        if [ "$IS_CONTAINER" -eq 1 ]; then
            rm -f "$CONTAINER_FLAG" 2>/dev/null || true
        else
            # On host
            rm -f "$HOST_FLAG_VOLUME" 2>/dev/null || true
            if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "psikologi_app"; then
                docker exec psikologi_app rm -f "$CONTAINER_FLAG" 2>/dev/null || true
            fi
            rm -f "$LOCAL_FLAG" 2>/dev/null || true
        fi

        echo ""
        echo "=========================================================="
        echo "  [AI GENERATION] STATUS: AKTIF (ON)"
        echo "=========================================================="
        echo "  AI telah diaktifkan kembali secara normal."
        echo "  Layanan generasi AI & Chat dapat digunakan kembali."
        echo "=========================================================="
        echo ""
        ;;

    status|check|cek)
        DISABLED=0
        if [ "$IS_CONTAINER" -eq 1 ]; then
            if [ -f "$CONTAINER_FLAG" ]; then
                DISABLED=1
            fi
        else
            if [ -f "$HOST_FLAG_VOLUME" ] || [ -f "$LOCAL_FLAG" ]; then
                DISABLED=1
            elif docker ps --format '{{.Names}}' 2>/dev/null | grep -q "psikologi_app"; then
                if docker exec psikologi_app test -f "$CONTAINER_FLAG" 2>/dev/null; then
                    DISABLED=1
                fi
            fi
        fi

        echo ""
        echo "=========================================================="
        if [ "$DISABLED" -eq 1 ]; then
            echo "  STATUS AI SAAT INI: [ NONAKTIF / MATI ]"
            echo "  Respon saat ini   : 'Token AI sudah habis'"
        else
            echo "  STATUS AI SAAT INI: [ AKTIF / NORMAL ]"
            echo "  Respon saat ini   : Generasi normal via Gemini AI"
        fi
        echo "=========================================================="
        echo "  Perintah yang tersedia:"
        echo "    ai off    -> Matikan AI (output: Token AI sudah habis)"
        echo "    ai on     -> Nyalakan kembali AI"
        echo "    ai status -> Cek status AI saat ini"
        echo "=========================================================="
        echo ""
        ;;

    *)
        echo "Penggunaan: ai {on|off|status}"
        echo "  ai off    - Matikan AI (output: Token AI sudah habis)"
        echo "  ai on     - Hidupkan kembali AI normal"
        echo "  ai status - Periksa status aktif/nonaktif AI saat ini"
        exit 1
        ;;
esac
