/**
 * Event Model (Combined)
 * Combines EventOnchain and EventMetadata for complete event data
 */

const EventOnchain = require('./EventOnchain');
const EventMetadata = require('./EventMetadata');

class Event {
    constructor(data = {}) {
        // Onchain data from Goldsky
        this.onchain = new EventOnchain(data.onchain || data);

        // Metadata from database
        this.metadata = new EventMetadata(data.metadata || data);

        // Combined convenience properties
        this.event_id = this.onchain.event_id || this.metadata.event_id;
        this.vault_address = this.onchain.vault;
        this.organizer_address = this.onchain.organizer;
        this.stake_amount = this.onchain.stake_amount;
        this.max_participants = this.onchain.max_participant;
    }

    /**
     * Get complete event data as JSON
     */
    toJSON() {
        return {
            event_id: this.event_id,
            vault_address: this.vault_address,
            organizer_address: this.organizer_address,
            stake_amount: this.stake_amount,
            max_participants: this.max_participants,
            status: this.metadata.status,
            current_participants: this.metadata.current_participants,
            deposited_to_yield: this.metadata.deposited_to_yield,
            event_settled: this.metadata.event_settled,
            total_yield_earned: this.metadata.total_yield_earned,
            total_net_yield: this.metadata.total_net_yield,

            // Onchain data
            transaction_hash: this.onchain.transaction_hash,
            block_number: this.onchain.block_number,
            created_at_timestamp: this.onchain.getCreatedAt(),
            registration_deadline: this.onchain.getRegistrationDeadline(),
            event_date: this.onchain.getEventDate(),

            // Metadata
            title: this.metadata.title,
            description: this.metadata.description,
            image_url: this.metadata.image_url,
            organizer_profile_id: this.metadata.organizer_profile_id,
            location: this.metadata.location,
            category: this.metadata.category,
            tags: this.metadata.tags,

            // Timestamps
            created_at: this.metadata.created_at,
            updated_at: this.metadata.updated_at
        };
    }

    /**
     * Get event summary for listings
     */
    toSummaryJSON() {
        return {
            event_id: this.event_id,
            vault_address: this.vault_address,
            title: this.metadata.title,
            description: this.metadata.description,
            image_url: this.metadata.image_url,
            stake_amount: this.stake_amount,
            max_participants: this.max_participants,
            current_participants: this.metadata.current_participants,
            status: this.metadata.status,
            registration_deadline: this.onchain.getRegistrationDeadline(),
            event_date: this.onchain.getEventDate(),
            location: this.metadata.location,
            category: this.metadata.category,
            tags: this.metadata.tags,
            organizer_address: this.organizer_address
        };
    }

    /**
     * Check if registration is open
     */
    isRegistrationOpen() {
        const now = Date.now();
        const deadline = this.onchain.getRegistrationDeadline();
        return deadline ? now < deadline.getTime() : false;
    }

    /**
     * Check if event is full
     */
    isEventFull() {
        return this.metadata.current_participants >= this.max_participants;
    }

    /**
     * Check if user can register for this event
     */
    canUserRegister() {
        return this.isRegistrationOpen() && !this.isEventFull() && this.metadata.status === 'REGISTRATION_OPEN';
    }

    /**
     * Create Event from combined database data
     */
    static fromDatabaseData(onchainRow, metadataRow) {
        return new Event({
            onchain: EventOnchain.fromDatabaseRow(onchainRow),
            metadata: EventMetadata.fromDatabaseRow(metadataRow)
        });
    }

    /**
     * Create Event from Goldsky event and database metadata
     */
    static fromGoldskyAndMetadata(goldskyEvent, metadataRow) {
        return new Event({
            onchain: EventOnchain.fromGoldskyEvent(goldskyEvent),
            metadata: metadataRow ? EventMetadata.fromDatabaseRow(metadataRow) : null
        });
    }

    /**
     * Validate complete event data
     */
    validate() {
        const onchainValidation = this.onchain.validate();
        const metadataValidation = this.metadata.validate();

        return {
            isValid: onchainValidation.isValid && metadataValidation.isValid,
            errors: [...onchainValidation.errors, ...metadataValidation.errors]
        };
    }
}

module.exports = Event;