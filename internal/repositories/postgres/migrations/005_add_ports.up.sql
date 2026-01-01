DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                   WHERE table_name='instances' AND column_name='ports') THEN
        ALTER TABLE instances ADD COLUMN ports VARCHAR(255);
    END IF;
END $$;
