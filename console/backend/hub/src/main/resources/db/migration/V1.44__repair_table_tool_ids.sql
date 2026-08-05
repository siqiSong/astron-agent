-- Link only accepts alphanumeric tool IDs. Repair workflows saved with the
-- original underscore-based IDs so existing agents remain runnable.
UPDATE flow_tool_rel
SET tool_id = CASE tool_id
    WHEN 'tool@table_extract_v1' THEN 'tool@tableextractv1'
    WHEN 'tool@table_bar_v1' THEN 'tool@tablebarv1'
    WHEN 'tool@table_pie_v1' THEN 'tool@tablepiev1'
    WHEN 'tool@excel_generate_v1' THEN 'tool@excelgeneratev1'
    ELSE tool_id
END
WHERE tool_id IN (
    'tool@table_extract_v1', 'tool@table_bar_v1',
    'tool@table_pie_v1', 'tool@excel_generate_v1'
);

UPDATE workflow
SET data = REPLACE(REPLACE(REPLACE(REPLACE(
    data,
    'tool@table_extract_v1', 'tool@tableextractv1'),
    'tool@table_bar_v1', 'tool@tablebarv1'),
    'tool@table_pie_v1', 'tool@tablepiev1'),
    'tool@excel_generate_v1', 'tool@excelgeneratev1')
WHERE data LIKE '%tool@table\_extract\_v1%'
   OR data LIKE '%tool@table\_bar\_v1%'
   OR data LIKE '%tool@table\_pie\_v1%'
   OR data LIKE '%tool@excel\_generate\_v1%';

UPDATE workflow
SET published_data = REPLACE(REPLACE(REPLACE(REPLACE(
    published_data,
    'tool@table_extract_v1', 'tool@tableextractv1'),
    'tool@table_bar_v1', 'tool@tablebarv1'),
    'tool@table_pie_v1', 'tool@tablepiev1'),
    'tool@excel_generate_v1', 'tool@excelgeneratev1')
WHERE published_data LIKE '%tool@table\_extract\_v1%'
   OR published_data LIKE '%tool@table\_bar\_v1%'
   OR published_data LIKE '%tool@table\_pie\_v1%'
   OR published_data LIKE '%tool@excel\_generate\_v1%';

UPDATE flow_protocol_temp
SET biz_protocol = REPLACE(REPLACE(REPLACE(REPLACE(
    biz_protocol,
    'tool@table_extract_v1', 'tool@tableextractv1'),
    'tool@table_bar_v1', 'tool@tablebarv1'),
    'tool@table_pie_v1', 'tool@tablepiev1'),
    'tool@excel_generate_v1', 'tool@excelgeneratev1')
WHERE biz_protocol LIKE '%tool@table\_extract\_v1%'
   OR biz_protocol LIKE '%tool@table\_bar\_v1%'
   OR biz_protocol LIKE '%tool@table\_pie\_v1%'
   OR biz_protocol LIKE '%tool@excel\_generate\_v1%';

UPDATE flow_protocol_temp
SET sys_protocol = REPLACE(REPLACE(REPLACE(REPLACE(
    sys_protocol,
    'tool@table_extract_v1', 'tool@tableextractv1'),
    'tool@table_bar_v1', 'tool@tablebarv1'),
    'tool@table_pie_v1', 'tool@tablepiev1'),
    'tool@excel_generate_v1', 'tool@excelgeneratev1')
WHERE sys_protocol LIKE '%tool@table\_extract\_v1%'
   OR sys_protocol LIKE '%tool@table\_bar\_v1%'
   OR sys_protocol LIKE '%tool@table\_pie\_v1%'
   OR sys_protocol LIKE '%tool@excel\_generate\_v1%';
