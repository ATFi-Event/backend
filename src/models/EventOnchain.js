/**
 * EventOnchain Model
 * Represents events from Goldsky subgraph matching the new table structure
 */

class EventOnchain {
    constructor(data = {}) {
        // Goldsky fields
        this.vid = data.vid || null;
        this.block = data.block || null;
        this.id = data.id || null;
        this.block_number = data.block_number || null;
        this.timestamp = data.timestamp || null;
        this.transaction_hash = data.transaction_hash || null;
        this.contract_id = data.contract_id || null;
        this.event_id = data.event_id || null;
        this.vault = data.vault || null;
        this.organizer = data.organizer || null;
        this.stake_amount = data.stake_amount || null;
        this.max_participant = data.max_participant || null;
        this.registration_deadline = data.registration_deadline || null;
        this.event_date = data.event_date || null;
        this._gs_chain = data._gs_chain || null;
        this._gs_gid = data._gs_gid || null;

        // Computed fields
        this.created_at = data.created_at || null;
        this.updated_at = data.updated_at || null;
    }

    /**
     * Get the registration deadline as Date object
     */
    getRegistrationDeadline() {
        return this.registration_deadline ? new Date(this.registration_deadline * 1000) : null;
    }

    /**
     * Get the event date as Date object
     */
    getEventDate() {
        return this.event_date ? new Date(this.event_date * 1000) : null;
    }

    /**
     * Get the created timestamp as Date object
     */
    getCreatedAt() {
        return this.timestamp ? new Date(this.timestamp * 1000) : null;
    }

    /**
     * Convert to JSON for API responses
     */
    toJSON() {
        return {
            vid: this.vid,
            block: this.block,
            id: this.id,
            block_number: this.block_number,
            timestamp: this.timestamp,
            transaction_hash: this.transaction_hash,
            contract_id: this.contract_id,
            event_id: this.event_id,
            vault: this.vault,
            organizer: this.organizer,
            stake_amount: this.stake_amount,
            max_participant: this.max_participant,
            registration_deadline: this.registration_deadline,
            event_date: this.event_date,
            _gs_chain: this._gs_chain,
            _gs_gid: this._gs_gid,
            created_at: this.created_at,
            updated_at: this.updated_at,
            // Computed fields for frontend compatibility
            registration_deadline_date: this.getRegistrationDeadline(),
            event_date_date: this.getEventDate(),
            created_at_date: this.getCreatedAt()
        };
    }

    /**
     * Validate required fields
     */
    validate() {
        const errors = [];

        if (!this.event_id) errors.push('event_id is required');
        if (!this.vault) errors.push('vault is required');
        if (!this.organizer) errors.push('organizer is required');
        if (!this.stake_amount) errors.push('stake_amount is required');
        if (!this.max_participant) errors.push('max_participant is required');
        if (!this.registration_deadline) errors.push('registration_deadline is required');
        if (!this.event_date) errors.push('event_date is required');
        if (!this.transaction_hash) errors.push('transaction_hash is required');

        return {
            isValid: errors.length === 0,
            errors
        };
    }

    /**
     * Create EventOnchain from Goldsky event data
     */
    static fromGoldskyEvent(goldskyEvent) {
        return new EventOnchain({
            vid: goldskyEvent.vid,
            block: goldskyEvent.block,
            id: goldskyEvent.id,
            block_number: goldskyEvent.block_number,
            timestamp: goldskyEvent.timestamp,
            transaction_hash: goldskyEvent.transaction_hash,
            contract_id: goldskyEvent.contract_id,
            event_id: goldskyEvent.event_id,
            vault: goldskyEvent.vault,
            organizer: goldskyEvent.organizer,
            stake_amount: goldskyEvent.stake_amount,
            max_participant: goldskyEvent.max_participant,
            registration_deadline: goldskyEvent.registration_deadline,
            event_date: goldskyEvent.event_date,
            _gs_chain: goldskyEvent._gs_chain,
            _gs_gid: goldskyEvent._gs_gid
        });
    }

    /**
     * Create EventOnchain from database row
     */
    static fromDatabaseRow(row) {
        return new EventOnchain({
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
            _gs_gid: row._gs_gid,
            created_at: row.created_at,
            updated_at: row.updated_at
        });
    }
}

module.exports = EventOnchain;