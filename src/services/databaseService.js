/**
 * Database Service for Goldsky Integration
 * Handles database operations with the new Goldsky table structure
 */

const { Pool } = require('pg');
const EventOnchain = require('../models/EventOnchain');
const EventMetadata = require('../models/EventMetadata');
const Event = require('../models/Event');

class DatabaseService {
    constructor() {
        this.pool = new Pool({
            connectionString: process.env.DATABASE_URL,
            ssl: process.env.NODE_ENV === 'production' ? { rejectUnauthorized: false } : false
        });
    }

    /**
     * Initialize database connection
     */
    async initialize() {
        try {
            await this.pool.query('SELECT NOW()');
            console.log('✅ Database connected successfully');
        } catch (error) {
            console.error('❌ Database connection failed:', error);
            throw error;
        }
    }

    /**
     * Insert or update event from Goldsky
     */
    async upsertEventFromGoldsky(goldskyEvent) {
        const client = await this.pool.connect();
        try {
            await client.query('BEGIN');

            // Insert into events_onchain (Goldsky structure)
            const upsertOnchainQuery = `
                INSERT INTO events_onchain (
                    vid, block, id, block_number, timestamp, transaction_hash,
                    contract_id, event_id, vault, organizer, stake_amount,
                    max_participant, registration_deadline, event_date,
                    _gs_chain, _gs_gid
                ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
                ON CONFLICT (transaction_hash) DO UPDATE SET
                    updated_at = NOW()
                RETURNING *
            `;

            const onchainValues = [
                goldskyEvent.vid,
                goldskyEvent.block,
                goldskyEvent.id,
                goldskyEvent.block_number,
                goldskyEvent.timestamp,
                goldskyEvent.transaction_hash,
                goldskyEvent.contract_id,
                goldskyEvent.event_id,
                goldskyEvent.vault,
                goldskyEvent.organizer,
                goldskyEvent.stake_amount,
                goldskyEvent.max_participant,
                goldskyEvent.registration_deadline,
                goldskyEvent.event_date,
                goldskyEvent._gs_chain,
                goldskyEvent._gs_gid
            ];

            const onchainResult = await client.query(upsertOnchainQuery, onchainValues);

            // Check if metadata exists, create default if not
            const metadataCheckQuery = 'SELECT * FROM events_metadata WHERE event_id = $1';
            const metadataResult = await client.query(metadataCheckQuery, [goldskyEvent.event_id]);

            if (metadataResult.rows.length === 0) {
                // Create default metadata
                const defaultTitle = `Event ${goldskyEvent.event_id}`;
                const insertMetadataQuery = `
                    INSERT INTO events_metadata (event_id, title, organizer_profile_id)
                    VALUES ($1, $2, $3)
                    ON CONFLICT (event_id) DO NOTHING
                `;

                // Try to find organizer profile by wallet address
                const organizerProfileQuery = 'SELECT id FROM profiles WHERE wallet_address = $1 LIMIT 1';
                const organizerResult = await client.query(organizerProfileQuery, [goldskyEvent.organizer]);
                const organizerProfileId = organizerResult.rows[0]?.id || null;

                await client.query(insertMetadataQuery, [
                    goldskyEvent.event_id,
                    defaultTitle,
                    organizerProfileId
                ]);
            }

            await client.query('COMMIT');

            return EventOnchain.fromDatabaseRow(onchainResult.rows[0]);
        } catch (error) {
            await client.query('ROLLBACK');
            console.error('Error upserting Goldsky event:', error);
            throw error;
        } finally {
            client.release();
        }
    }

    /**
     * Get complete event data (onchain + metadata)
     */
    async getCompleteEvent(eventId) {
        const query = `
            SELECT
                eo.*,
                em.title,
                em.description,
                em.image_url,
                em.organizer_profile_id,
                em.location,
                em.category,
                em.tags,
                em.status,
                em.current_participants,
                em.deposited_to_yield,
                em.event_settled,
                em.total_yield_earned,
                em.total_net_yield,
                em.created_at as metadata_created_at,
                em.updated_at as metadata_updated_at
            FROM events_onchain eo
            LEFT JOIN events_metadata em ON eo.event_id = em.event_id::text
            WHERE eo.event_id = $1
        `;

        const result = await this.pool.query(query, [eventId]);

        if (result.rows.length === 0) {
            return null;
        }

        const row = result.rows[0];

        // Combine onchain and metadata data
        const onchainData = {
            vid: row.vid,
            block: row.block,
            id: row.id,
            block_number: row.block_number,
            timestamp: row.timestamp,
            transaction_hash: row.transaction_hash,
            contract_id: row.contract_id,
            event_id: row.event_id,
            vault: row.vault,
            organizer: row.organizer,
            stake_amount: row.stake_amount,
            max_participant: row.max_participant,
            registration_deadline: row.registration_deadline,
            event_date: row.event_date,
            _gs_chain: row._gs_chain,
            _gs_gid: row._gs_gid
        };

        const metadataData = {
            event_id: row.event_id,
            title: row.title,
            description: row.description,
            image_url: row.image_url,
            organizer_profile_id: row.organizer_profile_id,
            location: row.location,
            category: row.category,
            tags: row.tags,
            status: row.status,
            current_participants: row.current_participants,
            deposited_to_yield: row.deposited_to_yield,
            event_settled: row.event_settled,
            total_yield_earned: row.total_yield_earned,
            total_net_yield: row.total_net_yield,
            created_at: row.metadata_created_at,
            updated_at: row.metadata_updated_at
        };

        return Event.fromDatabaseData(onchainData, metadataData);
    }

    /**
     * Get events with filters for Goldsky data
     */
    async getEvents(filters = {}) {
        let query = `
            SELECT
                eo.*,
                em.title,
                em.description,
                em.image_url,
                em.organizer_profile_id,
                em.location,
                em.category,
                em.tags,
                em.status,
                em.current_participants,
                em.deposited_to_yield,
                em.event_settled,
                em.total_yield_earned,
                em.total_net_yield,
                em.created_at as metadata_created_at,
                em.updated_at as metadata_updated_at
            FROM events_onchain eo
            LEFT JOIN events_metadata em ON eo.event_id = em.event_id::text
            WHERE 1=1
        `;

        const values = [];
        let paramIndex = 1;

        // Add filters
        if (filters.status) {
            query += ` AND em.status = $${paramIndex}`;
            values.push(filters.status);
            paramIndex++;
        }

        if (filters.organizer) {
            query += ` AND eo.organizer = $${paramIndex}`;
            values.push(filters.organizer);
            paramIndex++;
        }

        if (filters.deposited_to_yield !== undefined) {
            query += ` AND em.deposited_to_yield = $${paramIndex}`;
            values.push(filters.deposited_to_yield);
            paramIndex++;
        }

        // Add ordering
        query += ` ORDER BY eo.timestamp DESC`;

        // Add pagination
        if (filters.limit) {
            query += ` LIMIT $${paramIndex}`;
            values.push(filters.limit);
            paramIndex++;

            if (filters.offset) {
                query += ` OFFSET $${paramIndex}`;
                values.push(filters.offset);
            }
        }

        const result = await this.pool.query(query, values);

        return result.rows.map(row => {
            const onchainData = {
                vid: row.vid,
                block: row.block,
                id: row.id,
                block_number: row.block_number,
                timestamp: row.timestamp,
                transaction_hash: row.transaction_hash,
                contract_id: row.contract_id,
                event_id: row.event_id,
                vault: row.vault,
                organizer: row.organizer,
                stake_amount: row.stake_amount,
                max_participant: row.max_participant,
                registration_deadline: row.registration_deadline,
                event_date: row.event_date,
                _gs_chain: row._gs_chain,
                _gs_gid: row._gs_gid
            };

            const metadataData = {
                event_id: row.event_id,
                title: row.title,
                description: row.description,
                image_url: row.image_url,
                organizer_profile_id: row.organizer_profile_id,
                location: row.location,
                category: row.category,
                tags: row.tags,
                status: row.status,
                current_participants: row.current_participants,
                deposited_to_yield: row.deposited_to_yield,
                event_settled: row.event_settled,
                total_yield_earned: row.total_yield_earned,
                total_net_yield: row.total_net_yield,
                created_at: row.metadata_created_at,
                updated_at: row.metadata_updated_at
            };

            return Event.fromDatabaseData(onchainData, metadataData);
        });
    }

    /**
     * Update event metadata
     */
    async updateEventMetadata(eventId, updateData) {
        const allowedFields = [
            'title', 'description', 'image_url', 'location', 'category', 'tags'
        ];

        const updateFields = [];
        const values = [eventId];
        let paramIndex = 2;

        Object.keys(updateData).forEach(key => {
            if (allowedFields.includes(key)) {
                updateFields.push(`${key} = $${paramIndex}`);
                values.push(updateData[key]);
                paramIndex++;
            }
        });

        if (updateFields.length === 0) {
            throw new Error('No valid fields to update');
        }

        updateFields.push(`updated_at = NOW()`);

        const query = `
            UPDATE events_metadata
            SET ${updateFields.join(', ')}
            WHERE event_id = $1
            RETURNING *
        `;

        const result = await this.pool.query(query, values);

        if (result.rows.length === 0) {
            throw new Error('Event metadata not found');
        }

        return EventMetadata.fromDatabaseRow(result.rows[0]);
    }

    /**
     * Update event status
     */
    async updateEventStatus(eventId, status) {
        const query = `
            UPDATE events_metadata
            SET status = $1, updated_at = NOW()
            WHERE event_id = $2
            RETURNING *
        `;

        const result = await this.pool.query(query, [status, eventId]);

        if (result.rows.length === 0) {
            throw new Error('Event metadata not found');
        }

        return EventMetadata.fromDatabaseRow(result.rows[0]);
    }

    /**
     * Mark event as deposited to yield
     */
    async markEventAsDepositedToYield(eventId) {
        const query = `
            UPDATE events_metadata
            SET deposited_to_yield = true, updated_at = NOW()
            WHERE event_id = $1
            RETURNING *
        `;

        const result = await this.pool.query(query, [eventId]);

        if (result.rows.length === 0) {
            throw new Error('Event metadata not found');
        }

        return EventMetadata.fromDatabaseRow(result.rows[0]);
    }

    /**
     * Close database connection
     */
    async close() {
        await this.pool.end();
        console.log('📴 Database connection closed');
    }
}

module.exports = DatabaseService;