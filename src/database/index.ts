import { Client } from "cassandra-driver";
import { SCYLLA_CONTACT_POINTS, SCYLLA_DATA_CENTER, SCYLLA_KEYSPACE, SCYLLA_PASSWORD, SCYLLA_USERNAME } from "../../config";
import { createClient } from 'redis';
import { Logger } from "../helpers/logger";

const cassandra = new Client({
    contactPoints: JSON.parse(SCYLLA_CONTACT_POINTS),
    localDataCenter: SCYLLA_DATA_CENTER,
    keyspace: SCYLLA_KEYSPACE,
    credentials: (SCYLLA_USERNAME && SCYLLA_PASSWORD) ? {
        username: SCYLLA_USERNAME,
        password: SCYLLA_PASSWORD
    } : undefined
});

const redis = createClient();

const init = async () => {
    //cassandra.execute(`DROP TABLE strafechattesting.users;`)
    await cassandra.connect().catch(Logger.error);
    await redis.connect().catch(Logger.error);
}

export { cassandra, redis };
export default { init };