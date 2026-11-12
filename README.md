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
| **Painel** | Saldo das contas, entradas e saídas do mês, gastos por categoria, faturas abertas e parcelas a vencer. |
| **Transações** | Tudo que entrou e saiu, com busca, filtro por conta e categoria e troca de categoria em um clique. |
| **Orçamento** | Limite mensal por categoria, com barra de quanto já foi usado. |
| **Categorias e regras** | Suas categorias e regras por palavra-chave ("uber" → Transporte). Uma regra nova recategoriza o passado; sua escolha manual nunca é sobrescrita por uma sincronização. |
| **Conexões** | Conectar, sincronizar e remover bancos; mostra o vencimento do consentimento. |
| **Assistente** | Perguntas em linguagem natural: "posso gastar R$ 500 este mês?", "onde gastei mais?". |
| **Notificações** | Orçamento estourado, fatura vencendo, gasto grande fora do padrão, consentimento perto de vencer, conexão com erro e resumo do mês. |

Uma rotina diária sincroniza os bancos e gera os alertas. Contas, sessões, papéis e auditoria vêm da base de identidade do projeto (veja [ARCHITECTURE.md](ARCHITECTURE.md)).

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
make dev             # http://localhost:3006
```

A API responde em `API_PORT` (9979 por padrão). Mudou o `.env` do backend? Recrie os containers com `make compose-down && make compose-build`, pois o arquivo é lido na criação (os dados ficam no volume).

### 4. Conectar o banco

1. Em [meu.pluggy.ai](https://meu.pluggy.ai), conecte seus bancos e autorize o compartilhamento.
2. No Fin, abra **Conexões → Conectar banco → MeuPluggy**, entre com o login do MeuPluggy e autorize.
3. A primeira sincronização leva alguns minutos. Depois disso, o Fin atualiza sozinho todo dia; **Sincronizar agora** força na hora.

### Dados reais e custo

Conta trial da Pluggy só conecta bancos de teste (Pluggy Bank), com dados simulados. Para uso pessoal, o caminho que a própria Pluggy indica é o **MeuPluggy**: você conecta os bancos lá, de graça, e o Fin lê pelo conector MeuPluggy. Acesso de produção direto aos bancos exige empresa, due diligence e plano pago.

Limites conhecidos:

- O consentimento Open Finance vale 12 meses. O Fin avisa 15 dias antes; renove no MeuPluggy e sincronize.
- A Pluggy pode mudar as condições do MeuPluggy. As transações já sincronizadas continuam no seu banco de dados, mas **Remover** uma conexão apaga as contas e transações dela.
- Faça backup do Postgres (`pg_dump`); os dados vivem só no volume Docker.

## Desenvolvimento

- `backend/`: serviço Go (Fiber, GORM, Postgres), com `go.mod`, `Makefile`, migrações e testes próprios. `make dev-backend`, `make dev-mailer`, `make help`.
- `frontend/`: app Next.js que funciona como BFF. Veja [frontend/README.md](frontend/README.md).
- Testes com Postgres precisam de um banco vazio em `TEST_POSTGRES_URL`; sem ele, pulam:

```bash
TEST_POSTGRES_URL='postgres://postgres:test@localhost:5445/fin_test?sslmode=disable' go test -race ./...
```

Mais:

- [ARCHITECTURE.md](ARCHITECTURE.md): como as peças se encaixam, rotas, banco e segurança.
- [PRODUCT.md](PRODUCT.md): propósito e restrições.
- [TEMPLATE.md](TEMPLATE.md): como adicionar um módulo novo.

Diagnóstico interno em `DIAGNOSTICS_ADDRESS` (padrão `127.0.0.1:9091`, `/metrics` e `/health`). Não publique essa porta sem controle de acesso.
