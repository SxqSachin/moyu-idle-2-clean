-- Apply after the three 20260906 cattery migrations, with all old workers stopped.
BEGIN;
ALTER TABLE player_cattery_furniture ADD COLUMN IF NOT EXISTS effective_at timestamptz;
ALTER TABLE player_cattery_furniture ADD COLUMN IF NOT EXISTS previous_state jsonb;

CREATE OR REPLACE FUNCTION cattery_merge_contributions(previous jsonb, delta jsonb)
RETURNS jsonb LANGUAGE plpgsql IMMUTABLE STRICT AS $$
DECLARE
  result jsonb := previous;
  source_id text;
  addition jsonb;
  current_row jsonb;
  field_name text;
  counter_name text;
  metric_key text;
  metric_value jsonb;
  metrics jsonb;
  amount numeric;
BEGIN
  FOR source_id, addition IN SELECT * FROM jsonb_each(delta) LOOP
    current_row := COALESCE(result->source_id, '{}'::jsonb);
    FOREACH field_name IN ARRAY ARRAY['experience','items'] LOOP
      metrics := COALESCE(current_row->field_name, '{}'::jsonb);
      FOR metric_key, metric_value IN SELECT * FROM jsonb_each(COALESCE(addition->field_name,'{}'::jsonb)) LOOP
        amount := COALESCE((metrics->>metric_key)::numeric,0) + (metric_value #>> '{}')::numeric;
        IF field_name='experience' THEN amount := round(amount,4); END IF;
        IF amount<0 OR amount>1000000000000 THEN RAISE EXCEPTION 'furniture contribution overflow'; END IF;
        metrics := jsonb_set(metrics,ARRAY[metric_key],to_jsonb(amount));
      END LOOP;
      current_row := jsonb_set(current_row,ARRAY[field_name],metrics);
    END LOOP;
    FOREACH counter_name IN ARRAY ARRAY['opportunities','triggered','empty'] LOOP
      amount := COALESCE((current_row->>counter_name)::numeric,0) + COALESCE((addition->>counter_name)::numeric,0);
      IF amount<0 OR amount>1000000000000 THEN RAISE EXCEPTION 'furniture contribution overflow'; END IF;
      current_row := jsonb_set(current_row,ARRAY[counter_name],to_jsonb(amount));
    END LOOP;
    result := jsonb_set(result,ARRAY[source_id],current_row);
  END LOOP;
  RETURN result;
END;
$$;
COMMIT;
