/**
 * Event Controller for Goldsky Integration
 * Handles API endpoints with the new database structure
 */

const DatabaseService = require('../services/databaseService');

class EventController {
    constructor() {
        this.dbService = new DatabaseService();
    }

    /**
     * Initialize controller
     */
    async initialize() {
        await this.dbService.initialize();
    }

    /**
     * Get all events with optional filters
     * GET /api/v1/events
     */
    async getEvents(req, res) {
        try {
            const {
                status,
                organizer,
                deposited_to_yield,
                limit = 50,
                offset = 0
            } = req.query;

            const filters = {
                status,
                organizer,
                deposited_to_yield: deposited_to_yield === 'true' ? true : deposited_to_yield === 'false' ? false : undefined,
                limit: parseInt(limit),
                offset: parseInt(offset)
            };

            const events = await this.dbService.getEvents(filters);

            res.json({
                success: true,
                data: {
                    events: events.map(event => event.toSummaryJSON()),
                    total: events.length
                }
            });
        } catch (error) {
            console.error('Error getting events:', error);
            res.status(500).json({
                success: false,
                error: 'Failed to retrieve events',
                message: error.message
            });
        }
    }

    /**
     * Get single event by ID
     * GET /api/v1/events/:eventId
     */
    async getEvent(req, res) {
        try {
            const { eventId } = req.params;

            if (!eventId) {
                return res.status(400).json({
                    success: false,
                    error: 'Event ID is required'
                });
            }

            const event = await this.dbService.getCompleteEvent(eventId);

            if (!event) {
                return res.status(404).json({
                    success: false,
                    error: 'Event not found'
                });
            }

            res.json({
                success: true,
                data: event.toJSON()
            });
        } catch (error) {
            console.error('Error getting event:', error);
            res.status(500).json({
                success: false,
                error: 'Failed to retrieve event',
                message: error.message
            });
        }
    }

    /**
     * Create or update event metadata
     * POST /api/v1/events/:eventId/metadata
     */
    async upsertEventMetadata(req, res) {
        try {
            const { eventId } = req.params;
            const {
                title,
                description,
                image_url,
                location,
                category,
                tags
            } = req.body;

            if (!eventId) {
                return res.status(400).json({
                    success: false,
                    error: 'Event ID is required'
                });
            }

            // Check if event exists in onchain data first
            const existingEvent = await this.dbService.getCompleteEvent(eventId);
            if (!existingEvent) {
                return res.status(404).json({
                    success: false,
                    error: 'Event not found in onchain data'
                });
            }

            const updateData = {
                title,
                description,
                image_url,
                location,
                category,
                tags
            };

            // Remove undefined values
            Object.keys(updateData).forEach(key => {
                if (updateData[key] === undefined) {
                    delete updateData[key];
                }
            });

            const updatedMetadata = await this.dbService.updateEventMetadata(eventId, updateData);

            res.json({
                success: true,
                data: updatedMetadata.toJSON()
            });
        } catch (error) {
            console.error('Error upserting event metadata:', error);
            res.status(500).json({
                success: false,
                error: 'Failed to update event metadata',
                message: error.message
            });
        }
    }

    /**
     * Update event status
     * PUT /api/v1/events/:eventId/status
     */
    async updateEventStatus(req, res) {
        try {
            const { eventId } = req.params;
            const { status } = req.body;

            if (!eventId || !status) {
                return res.status(400).json({
                    success: false,
                    error: 'Event ID and status are required'
                });
            }

            const validStatuses = ['REGISTRATION_OPEN', 'REGISTRATION_CLOSED', 'LIVE', 'SETTLED', 'VOIDED'];
            if (!validStatuses.includes(status)) {
                return res.status(400).json({
                    success: false,
                    error: `Invalid status. Must be one of: ${validStatuses.join(', ')}`
                });
            }

            const updatedMetadata = await this.dbService.updateEventStatus(eventId, status);

            res.json({
                success: true,
                data: updatedMetadata.toJSON()
            });
        } catch (error) {
            console.error('Error updating event status:', error);
            res.status(500).json({
                success: false,
                error: 'Failed to update event status',
                message: error.message
            });
        }
    }

    /**
     * Get events ready for yield deposit
     * GET /api/v1/events/ready-for-yield
     */
    async getEventsReadyForYield(req, res) {
        try {
            const events = await this.dbService.getEvents({
                deposited_to_yield: false
            });

            // Filter events where registration deadline has passed
            const readyEvents = events.filter(event => {
                const now = Date.now();
                const deadline = event.onchain.getRegistrationDeadline();
                return deadline && now >= deadline.getTime() && event.metadata.current_participants > 0;
            });

            res.json({
                success: true,
                data: {
                    events: readyEvents.map(event => ({
                        event_id: event.event_id,
                        vault_address: event.vault_address,
                        title: event.metadata.title,
                        current_participants: event.metadata.current_participants,
                        stake_amount: event.stake_amount,
                        total_staked: event.metadata.current_participants * parseFloat(event.stake_amount),
                        registration_deadline: event.onchain.getRegistrationDeadline()
                    })),
                    total: readyEvents.length
                }
            });
        } catch (error) {
            console.error('Error getting events ready for yield:', error);
            res.status(500).json({
                success: false,
                error: 'Failed to retrieve events ready for yield',
                message: error.message
            });
        }
    }

    /**
     * Webhook endpoint for Goldsky events
     * POST /api/v1/webhooks/goldsky
     */
    async handleGoldskyWebhook(req, res) {
        try {
            const { events } = req.body;

            if (!Array.isArray(events)) {
                return res.status(400).json({
                    success: false,
                    error: 'Invalid webhook payload'
                });
            }

            const processedEvents = [];

            for (const goldskyEvent of events) {
                try {
                    const savedEvent = await this.dbService.upsertEventFromGoldsky(goldskyEvent);
                    processedEvents.push({
                        event_id: savedEvent.event_id,
                        vault: savedEvent.vault,
                        status: 'processed'
                    });
                } catch (error) {
                    console.error(`Error processing Goldsky event ${goldskyEvent.event_id}:`, error);
                    processedEvents.push({
                        event_id: goldskyEvent.event_id,
                        status: 'error',
                        error: error.message
                    });
                }
            }

            res.json({
                success: true,
                data: {
                    processed_events: processedEvents,
                    total: processedEvents.length
                }
            });
        } catch (error) {
            console.error('Error handling Goldsky webhook:', error);
            res.status(500).json({
                success: false,
                error: 'Failed to process webhook',
                message: error.message
            });
        }
    }

    /**
     * Get event statistics
     * GET /api/v1/events/stats
     */
    async getEventStats(req, res) {
        try {
            const allEvents = await this.dbService.getEvents();

            const stats = {
                total_events: allEvents.length,
                by_status: {},
                total_participants: 0,
                total_value_locked: 0,
                events_ready_for_yield: 0
            };

            allEvents.forEach(event => {
                // Count by status
                const status = event.metadata.status;
                stats.by_status[status] = (stats.by_status[status] || 0) + 1;

                // Sum participants and value
                stats.total_participants += event.metadata.current_participants;
                stats.total_value_locked += event.metadata.current_participants * parseFloat(event.stake_amount);

                // Count events ready for yield
                if (!event.metadata.deposited_to_yield && event.metadata.current_participants > 0) {
                    const now = Date.now();
                    const deadline = event.onchain.getRegistrationDeadline();
                    if (deadline && now >= deadline.getTime()) {
                        stats.events_ready_for_yield++;
                    }
                }
            });

            res.json({
                success: true,
                data: stats
            });
        } catch (error) {
            console.error('Error getting event stats:', error);
            res.status(500).json({
                success: false,
                error: 'Failed to retrieve event statistics',
                message: error.message
            });
        }
    }

    /**
     * Close database connections
     */
    async close() {
        await this.dbService.close();
    }
}

module.exports = EventController;