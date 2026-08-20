# Endpoints

Nenhum. `api/swagger.yaml` tem `paths: {}`. Base path declarado: `/v1`. Security: Bearer JWT no header `Authorization`.

Comentários SQL mencionam `POST /activate` como o passo que grava a senha. Esse handler não existe. Não tratar como contrato HTTP até haver adapter inbound.
