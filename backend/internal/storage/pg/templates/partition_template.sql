-- Create a sequence for autoincrement id
CREATE SEQUENCE %[1]s;

-- Create the partition with a DEFAULT using the sequence
CREATE TABLE %[2]s PARTITION OF %[3]s (
    id DEFAULT nextval(%[4]s)  -- Unique to this partition
)
FOR VALUES IN (%[5]s);

-- Associate the sequence with the column for automatic cleanup
ALTER SEQUENCE %[1]s OWNED BY %[2]s.id;
