# Примеры топологий баз данных

В этой директории содержатся примеры конфигураций топологий баз данных в формате YAML. Эти примеры используются для тестирования и демонстрации возможностей создания кластеров PostgreSQL с различными настройками и дополнениями (addons).

## Основные типы топологий

Конфигурация может описывать либо одиночный инстанс, либо кластер.

### 1. Одиночный инстанс (PostgresInstance)

Самый простой вариант развертывания. Описывает один сервер PostgreSQL.

**Пример:** `instance-basic.yaml`

```yaml
postgresInstance:
  settings:
    version: VERSION_17
    storageEngine: STORAGE_ENGINE_HEAP
  hardware:
    cores: 4
    memory: 8
    disk: 100
```

#### Sidecars

Для одиночного инстанса можно настроить дополнительные контейнеры (sidecars), такие как экспортеры метрик и бэкапы.

**Пример:** `instance-with-all-sidecars.yaml`

```yaml
postgresInstance:
  sidecars:
    - nodeExporter:
        enabled: true
        port: 9100
    - postgresExporter:
        enabled: true
        port: 9187
    - backup:
        schedule: "0 2 * * *"
        retention: 7d
        tool: WAL_G
```

### 2. Кластер (PostgresCluster)

Более сложная конфигурация, включающая мастер и реплики. Позволяет настраивать оборудование отдельно для мастера и реплик, а также количество реплик.

**Пример:** `cluster-base-r3.yaml`

```yaml
postgresCluster:
  topology:
    settings: ...
    masterHardware: ...
    replicaHardware: ...
    replicasCount: 3
```

## Дополнения (Addons) для кластера

Кластеры могут быть расширены с помощью различных дополнений, таких как DCS (Etcd), пулинг соединений (PgBouncer) и резервное копирование.

### DCS (Etcd)

Используется для управления состоянием кластера и выбора лидера (например, с Patroni).

**Способы размещения (Placement):**

*   **Dedicated (Выделенный):** Etcd разворачивается на отдельных инстансах.
    *   Пример: `cluster-etcd-dedicated-3.yaml`
    *   Требует указания `instancesCount` и `hardware`.
*   **Colocate (Совмещенный):** Etcd разворачивается на тех же узлах, что и PostgreSQL.
    *   `SCOPE_MASTER`: Только на мастере (пример: `cluster-etcd-colocate-master-r2.yaml`).
    *   `SCOPE_REPLICAS`: Только на репликах (пример: `cluster-etcd-colocate-replicas-r3.yaml`).
    *   `SCOPE_ALL_NODES`: На всех узлах (пример: `cluster-etcd-colocate-all-nodes-r2.yaml`).
    *   `SCOPE_REPLICA`: На конкретной реплике (примеры: `cluster-etcd-colocate-replica-*-r5.yaml`).

### Пулинг соединений (PgBouncer)

Обеспечивает управление пулом соединений к базе данных.

**Способы размещения (Placement):**

*   **Dedicated (Выделенный):** PgBouncer на отдельных машинах.
    *   Пример: `cluster-pgbouncer-dedicated-3.yaml`
*   **Colocate (Совмещенный):** PgBouncer на узлах PostgreSQL.
    *   Аналогично Etcd, поддерживает различные scopes (`SCOPE_MASTER`, `SCOPE_REPLICAS`, `SCOPE_ALL_NODES` и т.д.).
    *   Примеры: `cluster-pgbouncer-colocate-master-r2.yaml`, `cluster-pgbouncer-colocate-replicas-r3.yaml`.
    *   Также поддерживается размещение на конкретной реплике (`SCOPE_REPLICA`), пример: `cluster-pgbouncer-colocate-replica-2-r5.yaml`.

### Резервное копирование (Backup)

Настраивает параметры бэкапов (расписание, инструмент, хранилище).

**Пример:** `cluster-backup-scope-master-r2.yaml`

```yaml
backup:
  enabled: true
  scope: SCOPE_MASTER
  config:
    schedule: "0 3 * * *"
    retention: 7d
    tool: WAL_G
    local:
      path: /var/backups/postgres
```

Поддерживаются различные области действия (scopes): `SCOPE_MASTER`, `SCOPE_REPLICAS`, `SCOPE_ALL_NODES`.

## Patroni

В кластерных конфигурациях можно включить и настроить Patroni для высокой доступности.

**Пример:** `cluster-patroni-sync-2-r2.yaml`

```yaml
postgresCluster:
  topology:
    settings:
      patroni:
        enabled: true
        synchronousMode: true
        synchronousNodeCount: 2
```

Patroni часто используется совместно с Etcd. Пример конфигурации с Patroni и Etcd на всех узлах: `cluster-patroni-etcd-all-nodes-r2.yaml`.

## Переопределение настроек реплик (Replica Overrides)

Позволяет задавать специфические настройки для конкретных реплик в кластере.

**Пример:** `cluster-replica-override-r3.yaml`

```yaml
postgresCluster:
  replicaOverrides:
    - replicaIndex: 1
      hardware:
        cores: 8
        memory: 16
        disk: 200
    - replicaIndex: 2
      settings:
        version: VERSION_16
        storageEngine: STORAGE_ENGINE_ORIOLEDB
```

## Мониторинг

Включение мониторинга для кластера.

**Пример:** `cluster-monitor-r2.yaml`

```yaml
postgresCluster:
  topology:
    monitor: true
```

## Комбинированные топологии

Можно комбинировать несколько дополнений в одной конфигурации.

**Пример:** `cluster-combined-dedicated-r2.yaml`
Включает в себя Etcd (dedicated), PgBouncer (dedicated) и настроенный Backup.

**Пример:** `cluster-combined-colocate-all-r2.yaml`
Демонстрирует сложную конфигурацию с Patroni, Etcd (colocate all nodes), PgBouncer (colocate master), Backup (all nodes) и мониторингом.

## Тестирование

Файл `topologies_examples_test.go` содержит тест, который автоматически сканирует эту директорию на наличие `.yaml` файлов, парсит их и валидирует структуру шаблона базы данных. Это гарантирует, что все примеры остаются актуальными и корректными.
