/**
 * EventMetadata Model
 * Represents event metadata with status moved from events_onchain
 */

class EventMetadata {
    constructor(data = {}) {
        this.event_id = data.event_id || null;
        this.title = data.title || null;
        this.description = data.description || null;
        this.image_url = data.image_url || null;
        this.organizer_profile_id = data.organizer_profile_id || null;
        this.location = data.location || null;
        this.category = data.category || null;
        this.tags = data.tags || [];

        // Status moved from events_onchain
        this.status = data.status || 'REGISTRATION_OPEN';

        // Additional tracking fields
        this.current_participants = data.current_participants || 0;
        this.deposited_to_yield = data.deposited_to_yield || false;
        this.event_settled = data.event_settled || false;
        this.total_yield_earned = data.total_yield_earned || 0;
        this.total_net_yield = data.total_net_yield || 0;

        this.created_at = data.created_at || null;
        this.updated_at = data.updated_at || null;
    }

    /**
     * Convert to JSON for API responses
     */
    toJSON() {
        return {
            event_id: this.event_id,
            title: this.title,
            description: this.description,
            image_url: this.image_url,
            organizer_profile_id: this.organizer_profile_id,
            location: this.location,
            category: this.category,
            tags: this.tags,
            status: this.status,
            current_participants: this.current_participants,
            deposited_to_yield: this.deposited_to_yield,
            event_settled: this.event_settled,
            total_yield_earned: this.total_yield_earned,
            total_net_yield: this.total_net_yield,
            created_at: this.created_at,
            updated_at: this.updated_at
        };
    }

    /**
     * Validate required fields
     */
    validate() {
        const errors = [];

        if (!this.event_id) errors.push('event_id is required');
        if (!this.title) errors.push('title is required');
        if (!this.organizer_profile_id) errors.push('organizer_profile_id is required');

        // Validate status
        const validStatuses = ['REGISTRATION_OPEN', 'REGISTRATION_CLOSED', 'LIVE', 'SETTLED', 'VOIDED'];
        if (!validStatuses.includes(this.status)) {
            errors.push(`Invalid status. Must be one of: ${validStatuses.join(', ')}`);
        }

        return {
            isValid: errors.length === 0,
            errors
        };
    }

    /**
     * Update status based on current time and deadlines
     */
    updateStatusBasedOnTime(registrationDeadline, eventDate) {
        const now = Math.floor(Date.now() / 1000);

        if (now < registrationDeadline) {
            this.status = 'REGISTRATION_OPEN';
        } else if (now < eventDate) {
            this.status = this.current_participants > 0 ? 'REGISTRATION_CLOSED' : 'VOIDED';
        } else {
            this.status = 'LIVE';
        }

        this.updated_at = new Date().toISOString();
    }

    /**
     * Mark as deposited to yield
     */
    markAsDepositedToYield() {
        this.deposited_to_yield = true;
        this.updated_at = new Date().toISOString();
    }

    /**
     * Mark as settled
     */
    markAsSettled(totalYieldEarned, totalNetYield) {
        this.event_settled = true;
        this.total_yield_earned = totalYieldEarned;
        this.total_net_yield = totalNetYield;
        this.status = 'SETTLED';
        this.updated_at = new Date().toISOString();
    }

    /**
     * Increment participant count
     */
    incrementParticipantCount() {
        this.current_participants += 1;
        this.updated_at = new Date().toISOString();
    }

    /**
     * Decrement participant count
     */
    decrementParticipantCount() {
        if (this.current_participants > 0) {
            this.current_participants -= 1;
            this.updated_at = new Date().toISOString();
        }
    }

    /**
     * Create EventMetadata from database row
     */
    static fromDatabaseRow(row) {
        return new EventMetadata({
            event_id: row.event_id,
            title: row.title,
            description: row.description,
            image_url: row.image_url,
            organizer_profile_id: row.organizer_profile_id,
            location: row.location,
            category: row.category,
            tags: row.tags || [],
            status: row.status,
            current_participants: row.current_participants,
            deposited_to_yield: row.deposited_to_yield,
            event_settled: row.event_settled,
            total_yield_earned: row.total_yield_earned,
            total_net_yield: row.total_net_yield,
            created_at: row.created_at,
            updated_at: row.updated_at
        });
    }

    /**
     * Create EventMetadata from API request
     */
    static fromApiRequest(eventId, requestData) {
        return new EventMetadata({
            event_id: eventId,
            title: requestData.title,
            description: requestData.description,
            image_url: requestData.image_url,
            organizer_profile_id: requestData.organizer_profile_id,
            location: requestData.location,
            category: requestData.category,
            tags: requestData.tags || []
        });
    }
}

module.exports = EventMetadata;