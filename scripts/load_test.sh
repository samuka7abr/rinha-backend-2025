#!/bin/bash

set -e

API_URL="http://localhost:9999"
TOTAL_REQUESTS=50
CONCURRENT_REQUESTS=5
SLEEP_BETWEEN_BATCHES=0.1

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔥 INICIANDO STRESS TEST BRUTAL DA RINHA${NC}"
echo -e "${YELLOW}📊 Configurações:${NC}"
echo "   - Total de requisições: $TOTAL_REQUESTS"
echo "   - Requisições concorrentes: $CONCURRENT_REQUESTS"
echo "   - API: $API_URL"
echo ""

generate_uuid() {
    if command -v uuidgen &> /dev/null; then
        uuidgen | tr '[:upper:]' '[:lower:]'
    elif command -v python3 &> /dev/null; then
        python3 -c "import uuid; print(str(uuid.uuid4()))"
    else
        echo "$(printf '%08x-%04x-%04x-%04x-%012x' $RANDOM$RANDOM $RANDOM $RANDOM $RANDOM $RANDOM$RANDOM$RANDOM)"
    fi
}

make_payment() {
    local uuid=$1
    local amount=$2
    local start_time=$(date +%s%3N)
    
    response=$(curl -s -w "\n%{http_code}\n%{time_total}" \
        -X POST "$API_URL/payments" \
        -H "Content-Type: application/json" \
        -d "{\"correlationId\": \"$uuid\", \"amount\": $amount}")
    
    local end_time=$(date +%s%3N)
    local duration=$((end_time - start_time))
    
    local http_code=$(echo "$response" | tail -n 2 | head -n 1)
    local time_total=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -2)
    
    if [[ "$http_code" == "200" ]]; then
        echo -e "${GREEN}✅ SUCCESS${NC} | $uuid | ${amount} | ${duration}ms | HTTP $http_code"
    else
        echo -e "${RED}❌ FAILED${NC}  | $uuid | ${amount} | ${duration}ms | HTTP $http_code"
        echo "   Body: $body"
    fi
    
    echo "$http_code,$duration" >> /tmp/payment_metrics.txt
}

test_duplicate() {
    local uuid=$1
    local amount=$2
    
    echo -e "${YELLOW}🔄 Testando duplicata: $uuid${NC}"
    
    make_payment "$uuid" "$amount"
    sleep 0.1
    
    echo -e "${BLUE}   → Enviando duplicata...${NC}"
    make_payment "$uuid" "$((amount + 100))"
}

show_redis_metrics() {
    echo -e "\n${BLUE}📊 MÉTRICAS DO REDIS:${NC}"
    
    echo "🏥 Health checks armazenados:"
    docker exec rinha-redis redis-cli KEYS "health:*" 2>/dev/null || echo "   Nenhum health check encontrado"
    
    echo "💰 Pagamentos cacheados:"
    local cached_payments=$(docker exec rinha-redis redis-cli KEYS "payment:*" 2>/dev/null | wc -l)
    echo "   Total: $cached_payments pagamentos em cache"
    
    for processor in "default" "fallback"; do
        local health=$(docker exec rinha-redis redis-cli GET "health:$processor" 2>/dev/null)
        if [[ -n "$health" && "$health" != "(nil)" ]]; then
            echo "🔧 Health $processor:"
            echo "   $health" | jq . 2>/dev/null || echo "   $health"
        fi
    done
}

show_db_stats() {
    echo -e "\n${BLUE}🗄️  ESTATÍSTICAS DO BANCO:${NC}"
    
    local total=$(docker exec rinha-postgres psql -U admin -d rinha -t -c "SELECT COUNT(*) FROM payments;" 2>/dev/null | tr -d ' ')
    echo "💾 Total de pagamentos salvos: $total"
    
    echo "📈 Resumo por processador:"
    docker exec rinha-postgres psql -U admin -d rinha -c "
        SELECT 
            processor,
            COUNT(*) as total,
            SUM(amount) as valor_total,
            AVG(amount) as valor_medio
        FROM payments 
        GROUP BY processor 
        ORDER BY total DESC;" 2>/dev/null || echo "   Erro ao consultar banco"
}

calculate_stats() {
    if [[ ! -f /tmp/payment_metrics.txt ]]; then
        echo "Nenhuma métrica coletada"
        return
    fi
    
    echo -e "\n${BLUE}⚡ ESTATÍSTICAS DE PERFORMANCE:${NC}"
    
    local total_requests=$(wc -l < /tmp/payment_metrics.txt)
    local success_count=$(grep "^200," /tmp/payment_metrics.txt | wc -l)
    local error_count=$((total_requests - success_count))
    
    echo "📊 Resumo geral:"
    echo "   Total de requisições: $total_requests"
    echo "   Sucessos: $success_count"
    echo "   Erros: $error_count"
    
    if [[ $success_count -gt 0 ]]; then
        echo "⏱️  Tempos de resposta (ms):"
        local times=$(grep "^200," /tmp/payment_metrics.txt | cut -d',' -f2 | sort -n)
        local avg=$(echo "$times" | awk '{sum+=$1} END {printf "%.1f", sum/NR}')
        local p50=$(echo "$times" | awk -v p=50 '{a[NR]=$1} END {print a[int(NR*p/100)+1]}')
        local p95=$(echo "$times" | awk -v p=95 '{a[NR]=$1} END {print a[int(NR*p/100)+1]}')
        local p99=$(echo "$times" | awk -v p=99 '{a[NR]=$1} END {print a[int(NR*p/100)+1]}')
        local max_time=$(echo "$times" | tail -n 1)
        local min_time=$(echo "$times" | head -n 1)
        
        echo "   Mínimo: ${min_time}ms"
        echo "   Médio: ${avg}ms"
        echo "   P50: ${p50}ms"
        echo "   P95: ${p95}ms"
        echo "   P99: ${p99}ms 🎯"
        echo "   Máximo: ${max_time}ms"
        
        if [[ $p99 -lt 11 ]]; then
            echo -e "   ${GREEN}🏆 OBJETIVO ATINGIDO! P99 < 11ms = BÔNUS!${NC}"
        else
            echo -e "   ${YELLOW}⚠️  P99 acima de 11ms, sem bônus de performance${NC}"
        fi
    fi
}

rm -f /tmp/payment_metrics.txt

echo -e "${YELLOW}🚀 Iniciando bombardeio...${NC}\n"

echo -e "${BLUE}📊 FASE 1: Carga Normal${NC}"
for i in $(seq 1 $((TOTAL_REQUESTS / 3))); do
    uuid=$(generate_uuid)
    amount=$((50 + RANDOM % 950))
    
    if [[ $((i % CONCURRENT_REQUESTS)) -eq 0 ]]; then
        wait
        sleep $SLEEP_BETWEEN_BATCHES
    fi
    
    make_payment "$uuid" "$amount" &
done
wait

echo -e "\n${BLUE}📊 FASE 2: Teste de Duplicatas${NC}"
uuid_dup=$(generate_uuid)
test_duplicate "$uuid_dup" 999

echo -e "\n${BLUE}📊 FASE 3: Bombardeio Intenso${NC}"
for i in $(seq 1 $((TOTAL_REQUESTS / 2))); do
    uuid=$(generate_uuid)
    amount=$((10 + RANDOM % 190))
    
    make_payment "$uuid" "$amount" &
    
    if [[ $((i % (CONCURRENT_REQUESTS * 2))) -eq 0 ]]; then
        wait
        sleep 0.05
    fi
done
wait

echo -e "\n${GREEN}🎯 BOMBARDEIO CONCLUÍDO!${NC}"

show_redis_metrics
show_db_stats
calculate_stats

echo -e "\n${GREEN}🏁 TESTE FINALIZADO!${NC}"
echo -e "${BLUE}💡 Para ver logs detalhados: docker logs app-1${NC}"
echo -e "${BLUE}💡 Para ver Redis: docker exec -it rinha-redis redis-cli KEYS '*'${NC}" 