import argparse
import datetime
import re
import secrets
import subprocess
import time


def main():
    # 仅对独立容器中的虚构验收数据库执行分区增删，不允许连接业务库。
    parser = argparse.ArgumentParser()
    parser.add_argument('--container', required=True)
    parser.add_argument('--database', required=True)
    parser.add_argument('--password', required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'[A-Za-z0-9_]+_user_test', args.database):
        raise RuntimeError('Partition checks require an isolated _user_test database')

    def sql(statement, expected_failure=False):
        result = subprocess.run(
            ['docker', 'exec', args.container, 'mysql', '-uroot',
             '--password=' + args.password, '--database=' + args.database,
             '--batch', '--skip-column-names', '--execute', statement],
            capture_output=True, text=True, check=False,
        )
        if expected_failure:
            if result.returncode == 0 or '1526' not in result.stderr:
                raise RuntimeError('Missing daily partition did not reject the audit write')
        elif result.returncode:
            raise RuntimeError('Partition verification SQL failed: ' + result.stderr)
        return result.stdout.strip()

    def count():
        return int(sql("SELECT COUNT(*) FROM information_schema.PARTITIONS "
                       "WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'audit_events'"))

    assert count() == 30, 'Initial SQL must create exactly 30 daily partitions'
    details = sql("SELECT CONCAT(PARTITION_METHOD, ':', PARTITION_NAME, ':', PARTITION_DESCRIPTION) "
                  "FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA = DATABASE() "
                  "AND TABLE_NAME = 'audit_events' ORDER BY PARTITION_ORDINAL_POSITION").splitlines()
    today = datetime.date.fromisoformat(sql("SELECT UTC_DATE()"))
    for index, detail in enumerate(details):
        day = today + datetime.timedelta(days=index)
        assert detail == "LIST COLUMNS:p" + day.strftime('%Y%m%d') + ":'" + day.isoformat() + "'", 'Unexpected daily partition boundary'
    assert sql("SELECT CONCAT(STATUS, ':', INTERVAL_VALUE, ':', INTERVAL_FIELD, ':', TIME_ZONE) "
               "FROM information_schema.EVENTS WHERE EVENT_SCHEMA = DATABASE() "
               "AND EVENT_NAME = 'maintain_audit_events_partitions_daily'") == 'ENABLED:1:DAY:+00:00'
    assert sql("SELECT SECURITY_TYPE FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = DATABASE() "
               "AND ROUTINE_NAME = 'maintain_audit_events_partitions'") == 'DEFINER'
    assert sql("SELECT DATE_FORMAT(STARTS, '%H:%i') FROM information_schema.EVENTS "
               "WHERE EVENT_SCHEMA = DATABASE() AND EVENT_NAME = 'maintain_audit_events_partitions_daily'") == '00:05'
    assert sql('SELECT @@global.event_scheduler') == 'ON', 'MySQL event_scheduler must be enabled by operations'
    sql('CALL maintain_audit_events_partitions(); CALL maintain_audit_events_partitions();')
    assert count() == 30, 'Partition maintenance must be idempotent'

    # 克隆虚构验收记录，仅替换编号和日期，验证分区写入路由，不复制真实数据。
    columns = (
        'workspace_id,event_content,action_id,request_id,user_id,user_identify_type,'
        'user_identify_tenant_id,access_type,access_source_ip,access_user_agent,resource_type_id,'
        'instance_id,instance_name,instance_data,instance_origin_data,result_code,result_content,'
        'snapshot_user_info,extend_data,sensitive_snapshot,sensitive_key_id,sensitive_context'
    )
    assert int(sql('SELECT COUNT(*) FROM audit_events')) > 0, 'Run user integration fixtures first'

    def insert_for(day, expected_failure=False):
        identifier = secrets.token_hex(16)
        start = int(datetime.datetime.combine(
            day, datetime.time(), tzinfo=datetime.timezone.utc,
        ).timestamp() * 1000)
        sql("INSERT INTO audit_events (event_id,event_date,start_time,end_time," + columns +
            ") SELECT '" + identifier + "','" + day.isoformat() + "'," + str(start) + "," +
            str(start) + "," + columns + ' FROM audit_events LIMIT 1', expected_failure)

    # 删除窗口中间一天也必须拒绝写入，不能落入相邻日分区。
    missing_day = today + datetime.timedelta(days=15)
    sql('ALTER TABLE audit_events DROP PARTITION p' + missing_day.strftime('%Y%m%d'))
    insert_for(missing_day, expected_failure=True)
    insert_for(today + datetime.timedelta(days=30), expected_failure=True)
    # 在独立测试库创建一次性 Event，验证调度器确实能以定义者权限调用维护过程。
    sql('CREATE EVENT verify_audit_events_partition_once ON SCHEDULE AT CURRENT_TIMESTAMP + INTERVAL 1 SECOND '
        'DO CALL maintain_audit_events_partitions()')
    for _ in range(10):
        if count() == 30:
            break
        time.sleep(1)
    assert count() == 30, 'Maintenance did not repair the missing exact day'
    insert_for(missing_day)

    # 仅调整当前验收连接的时钟，模拟次日调度；历史分区继续保留。
    sql('SET timestamp = UNIX_TIMESTAMP(UTC_TIMESTAMP() + INTERVAL 1 DAY); CALL maintain_audit_events_partitions();')
    assert count() == 31, 'Next-day maintenance did not advance the future 30-day window'
    insert_for(today + datetime.timedelta(days=30))
    insert_for(today + datetime.timedelta(days=31), expected_failure=True)
    print('Audit partition initialization, daily event, gap rejection, repair, and rolling window passed')


if __name__ == '__main__':
    main()
