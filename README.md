# Fin

Fin é um assistente de finanças pessoais **auto-hospedado**. Ele lê suas contas e cartões pelo Open Finance Brasil (via [Pluggy](https://pluggy.ai)), categoriza os gastos, acompanha orçamento e faturas, avisa quando algo pede atenção e responde perguntas sobre o seu dinheiro com um assistente de IA.

## Objetivo

Ver, num só lugar e sem planilha, **para onde o dinheiro vai e quanto ainda dá para gastar no mês**, com os dados reais do banco, atualizados todo dia, e sem entregar senha nem dados para um app de terceiros.

Princípios:

- **Só leitura.** O Fin nunca movimenta dinheiro. O acesso ao banco é um consentimento Open Finance que você revoga quando quiser.
- **Seus dados, sua máquina.** Tudo fica no seu Postgres. O assistente recebe só números já calculados (totais, categorias, faturas), nunca número de conta.
- **Uso pessoal.** Feito para uma pessoa ou família rodar para si, não como serviço para terceiros.

## O que ele faz

| Tela | Para quê |
|------|----------|
| **Painel** | Saldo das contas, patrimônio com os investimentos, entradas e saídas do mês, previsão até o fim do mês (sobra prevista e quanto dá para gastar por dia), últimos 12 meses, gastos por categoria, faturas (próximo vencimento e última fechada), contas recorrentes e parcelas a vencer. |
| **Transações** | Tudo que entrou e saiu, com busca, filtro por conta e categoria e troca de categoria em um clique. Uma compra pode ser **dividida** entre categorias, e o que não deve contar (uma cobrança duplicada, por exemplo) vai para **Ignoradas**. |
| **Próximos 30 dias** | Faturas, contas recorrentes e salário previstos, com o saldo da conta dia a dia e o aviso de quando ele ficaria negativo. |
| **Orçamento** | Limite mensal por categoria, com barra de quanto já foi usado. |
| **Assinaturas** | Assinaturas e contas fixas cadastradas (mensais ou anuais), com o status do mês (paga, a vencer, não apareceu) conferido no extrato e o custo por mês e por ano. Lista também as cobranças que se repetem e ainda não foram cadastradas: um clique cadastra, outro descarta. Entram na previsão e em Próximos 30 dias. |
| **Metas** | Objetivos de economia (reserva de emergência, viagem), com o progresso seguindo o saldo de uma conta ou um valor informado, e quanto guardar por mês até a data. |
| **Relatório anual** | O ano por categoria e por mês, com saúde e educação destacadas, para conferir a declaração do Imposto de Renda. |
| **Pessoas** | Pix e TED por pessoa nos últimos 12 meses: quanto enviou e recebeu. Um botão marca o que alguém te devolve como **reembolso**, que abate a despesa em vez de inflar a receita. |
| **Categorias e regras** | Suas categorias e regras por palavra-chave ("uber" → Transporte), com sugestões tiradas dos estabelecimentos que se repetem sem categoria. Uma regra nova recategoriza o passado; sua escolha manual nunca é sobrescrita por uma sincronização. |
| **Conexões** | Conectar, sincronizar e remover bancos; mostra o vencimento do consentimento. Para um banco que o Open Finance não alcança, **Importar extrato** cria uma conta e lê arquivos OFX ou CSV (Nubank e outros); reenviar o mesmo arquivo não duplica nada. |
| **Assistente** | Perguntas em linguagem natural: "posso gastar R$ 500 este mês?", "onde gastei mais?". |
| **Notificações** | Orçamento estourado, fatura vencendo, gasto grande fora do padrão, conta no negativo e cobrança de juros, cobrança repetida no mesmo dia, conta recorrente que ficou mais cara, consentimento perto de vencer, conexão com erro e resumo do mês. Com SMTP ligado, os avisos e os críticos também chegam por e-mail. |

Uma rotina diária sincroniza os bancos e gera os alertas.

A categorização olha, nesta ordem: suas regras, pagamentos de fatura, palavras da descrição (salário, rendimento, consignado, juros, IOF, seguro, assinaturas conhecidas) e por último a categoria que o banco manda, que costuma errar (salário como "Services", por exemplo). Empréstimos e transferências entre suas contas não contam como receita nem como gasto. Cada sincronização reaplica a categorização automática ao histórico; o que você escolheu à mão ou por regra fica como está. Contas, sessões, papéis e auditoria vêm da base de identidade do projeto (veja [ARCHITECTURE.md](ARCHITECTURE.md)).

## Uso

### 1. Pré-requisitos

- Docker e Docker Compose; Node 22+ e pnpm para o frontend.
- Uma conta em [dashboard.pluggy.ai](https://dashboard.pluggy.ai) (client id e secret).
- Uma conta em [meu.pluggy.ai](https://meu.pluggy.ai) com seus bancos conectados (veja "Dados reais" abaixo).
- Opcional, para o assistente: assinatura Claude (`claude setup-token`) ou qualquer API compatível com OpenAI, como o [Ollama](https://ollama.com) local.

### 2. Configurar

```bash
cd backend && make init     # cria config/.env com segredos novos
cd ../frontend && make init # cria config/.env do web
```

Preencha em `backend/config/.env` (cada opção tem comentário):

| Variável | O que é |
|----------|---------|
| `PLUGGY_CLIENT_ID`, `PLUGGY_CLIENT_SECRET` | Credenciais da Pluggy. Vazio desliga a sincronização. |
| `PLUGGY_WEBHOOK_URL`, `PLUGGY_WEBHOOK_SECRET` | Opcional: URL pública https para a Pluggy avisar de dados novos. Sem ela, a rotina diária sincroniza. |
| `AI_PROVIDER` | `claude`, `openai` ou vazio (assistente desligado). |
| `AI_MODEL` | Modelo: `sonnet`/`opus` no Claude; o nome do modelo no provedor OpenAI-compatível. |
| `CLAUDE_CODE_OAUTH_TOKEN` | Com `AI_PROVIDER=claude`: token do `claude setup-token`. |
| `AI_OPENAI_URL`, `AI_OPENAI_KEY` | Com `AI_PROVIDER=openai`: endereço e chave (o Ollama ignora a chave). |

Em `frontend/config/.env`, `NEXT_PUBLIC_PLUGGY_CONNECTORS='200'` mostra só o MeuPluggy no widget de conexão.

Os arquivos `.env` ficam fora do git. Nunca faça commit de segredos.

### 3. Subir

```bash
cd backend
make compose-build   # postgres, backend e mailer
make migrate-up      # schema e logins do banco
make admin-bootstrap # nome, e-mail e senha do primeiro usuário
```

```bash
cd frontend
make compose-build   # web e proxy HTTPS
make https-ca        # salva config/fin-root-ca.crt
```

Abra `https://<IP desta máquina>:3443` em qualquer aparelho da rede (nesta máquina, `https://localhost:3443` ou `http://localhost:3016`). O certificado vem de uma autoridade local criada pelo próprio proxy: instale `config/fin-root-ca.crt` como raiz confiável em cada aparelho para o navegador não reclamar e para o Fin poder ser **instalado como app** (PWA) no celular. Se o IP da máquina mudar, ajuste `WEB_HOSTS` e `WEB_SNI` em `frontend/config/.env` e rode `make compose-up`. Para desenvolver com recarga automática, `make dev` (porta 3016).

A API responde em `API_PORT` (9979 por padrão), só nesta máquina: os outros aparelhos passam pelo web. Mudou o `.env` do backend? Recrie os containers com `make compose-down && make compose-build`, pois o arquivo é lido na criação (os dados ficam no volume).

### 4. Conectar o banco

1. Em [meu.pluggy.ai](https://meu.pluggy.ai), conecte seus bancos e autorize o compartilhamento.
2. No Fin, abra **Conexões → Conectar banco → MeuPluggy**, entre com o login do MeuPluggy e autorize.
3. A primeira sincronização leva alguns minutos. Depois disso, o Fin atualiza sozinho todo dia; **Sincronizar agora** força na hora.

### Dados reais e custo

Conta trial da Pluggy só conecta bancos de teste (Pluggy Bank), com dados simulados. Para uso pessoal, o caminho que a própria Pluggy indica é o **MeuPluggy**: você conecta os bancos lá, de graça, e o Fin lê pelo conector MeuPluggy. Acesso de produção direto aos bancos exige empresa, due diligence e plano pago.

Limites conhecidos:

- O consentimento Open Finance vale 12 meses. O Fin avisa 15 dias antes; renove no MeuPluggy e sincronize.
- A Pluggy pode mudar as condições do MeuPluggy. As transações já sincronizadas continuam no seu banco de dados, mas **Remover** uma conexão apaga as contas e transações dela.
- O histórico vive no volume Docker do Postgres; veja "Backup" abaixo.

### Backup

O serviço `fin_backup` (sobe com `make compose-build`) grava todo dia um dump do banco em `backend/backups/` (`BACKUP_DIR`), um arquivo por dia da semana (`fin-1.dump` é segunda): ficam sempre os últimos 7 dias, sem apagar nada. Os arquivos têm todos os seus dados e ficam só para o root. Um disco só não é backup: copie-os para outro disco ou um NAS com `make backup-copy DEST=/media/hd/fin` (em `backend/`). Para fazer isso todo dia, uma linha no `sudo crontab -e`:

```
30 4 * * * cd /caminho/do/fin/backend && make backup-copy DEST=/media/hd/fin
```

Restaurar um dump (de `backend/`, com a stack no ar; troque `5445` pelo seu `POSTGRES_PORT`; substitui o banco atual):

```bash
sudo cat backups/fin-1.dump | docker exec -i fin_postgres sh -c 'pg_restore --clean --if-exists -p 5445 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

## Desenvolvimento

- `backend/`: serviço Go (Fiber, GORM, Postgres), com `go.mod`, `Makefile`, migrações e testes próprios. `make dev-backend`, `make dev-mailer`, `make help`.
- `frontend/`: app Next.js que funciona como BFF. Veja [frontend/README.md](frontend/README.md).
- Testes com Postgres precisam de um banco vazio em `TEST_POSTGRES_URL`; sem ele, pulam. `make test-integration` (em `backend/`, com a stack no ar) cria um `fin_test` limpo no `fin_postgres` e roda todos:

```bash
make test-integration
```

Mais:

- [ARCHITECTURE.md](ARCHITECTURE.md): como as peças se encaixam, rotas, banco e segurança.
- [PRODUCT.md](PRODUCT.md): propósito e restrições.
- [TEMPLATE.md](TEMPLATE.md): como adicionar um módulo novo.

Diagnóstico interno em `DIAGNOSTICS_ADDRESS` (padrão `127.0.0.1:9091`, `/metrics` e `/health`). Não publique essa porta sem controle de acesso.
